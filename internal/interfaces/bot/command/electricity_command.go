package command

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gamee1910/volt/config"
	"github.com/gamee1910/volt/internal/application"
	"github.com/gamee1910/volt/internal/common/utils"
	"github.com/gamee1910/volt/pkg/logger"
	"github.com/gamee1910/volt/pkg/telegram"
)

type ElectricityCommand struct {
	cfg            *config.Configuration
	log            *logger.Logger
	telegramClient telegram.TelegramClient
	service        *application.ElectricityService
}

func NewElectricityCommand(
	cfg *config.Configuration,
	log *logger.Logger,
	telegramClient telegram.TelegramClient,
	service *application.ElectricityService,
) *ElectricityCommand {
	return &ElectricityCommand{
		cfg:            cfg,
		log:            log,
		telegramClient: telegramClient,
		service:        service,
	}
}

func (c *ElectricityCommand) HandleYesterdayCommand(ctx context.Context, chatID int64) error {
	usage, err := c.service.Yesterday(ctx)
	if err != nil {
		var unavailable *application.UsageUnavailableError
		if errors.As(err, &unavailable) {
			return c.telegramClient.SendMessage(ctx, chatID, unavailable.Error())
		}
		c.log.Error("failed_to_get_yesterday_usage", map[string]any{"error": err.Error()})
		return c.telegramClient.SendMessage(ctx, chatID, "failed to fetch data: "+err.Error())
	}

	msg := fmt.Sprintf(
		"Điện năng ngày %s:\n Tiêu thụ: %.2f KWh\n Tổng tiền tháng này: %s",
		usage.MeasurementDate.Format("02/01/2006"),
		usage.ConsumptionKWh,
		utils.FormatVND(usage.TotalAmount),
	)
	return c.telegramClient.SendMessage(ctx, chatID, msg)
}

func (c *ElectricityCommand) HandleGetAllCommand(ctx context.Context, chatID int64) error {
	resp, err := c.service.CurrentMonth(ctx)
	if err != nil {
		c.log.Error("failed_to_get_current_month_usage", map[string]any{"error": err.Error()})
		return c.telegramClient.SendMessage(ctx, chatID, "Failed to fetch data: "+err.Error())
	}

	if len(resp.Data) == 0 {
		return c.telegramClient.SendMessage(ctx, chatID, "Chưa có dữ liệu sản lượng điện tháng này.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Sản lượng điện tháng này (%d ngày):\n\n", len(resp.Data)))
	for _, usage := range resp.Data {
		sb.WriteString(fmt.Sprintf("• Ngày %s: %.2f KWh | %s\n",
			usage.MeasurementDate.Format("02/01/2006"),
			usage.ConsumptionKWh,
			utils.FormatVND(usage.TotalAmount),
		))
	}
	sb.WriteString(fmt.Sprintf("\n Tổng tiêu thụ: %.2f KWh\n Tổng tiền tháng này: %s",
		resp.TotalKWh, utils.FormatVND(resp.TotalAmount),
	))

	msgText := sb.String()
	if len(msgText) > 4000 {
		return c.sendChunked(ctx, chatID, msgText)
	}
	return c.telegramClient.SendMessage(ctx, chatID, msgText)
}

func (c *ElectricityCommand) HandleSyncCommand(ctx context.Context, chatID int64, text string) error {
	parts := strings.Fields(text)
	var fromDate, toDate string
	if len(parts) >= 3 {
		fromDate, toDate = parts[1], parts[2]
	} else {
		var err error
		fromDate, toDate, err = c.service.CurrentMonthRange()
		if err != nil {
			return c.telegramClient.SendMessage(ctx, chatID, fmt.Sprintf("Sync failed: %s", err.Error()))
		}
	}

	if err := c.service.Sync(ctx, fromDate, toDate); err != nil {
		c.log.Error("failed_to_sync_evn_data", map[string]any{
			"from_date": fromDate,
			"to_date":   toDate,
			"error":     err.Error(),
		})
		return c.telegramClient.SendMessage(ctx, chatID, fmt.Sprintf("Sync failed: %s", err.Error()))
	}

	return c.telegramClient.SendMessage(ctx, chatID, fmt.Sprintf("Đồng bộ dữ liệu thành công từ %s đến %s!", fromDate, toDate))
}

func (c *ElectricityCommand) NotifySyncResult(ctx context.Context, syncErr error) error {
	chatID := c.cfg.ApplicationConfig.TelegramConfig.ChatID
	if chatID == 0 {
		return fmt.Errorf("TELEGRAM_CHAT_ID is required to send sync notifications")
	}
	if syncErr != nil {
		return c.telegramClient.SendMessage(ctx, chatID, "Đồng bộ dữ liệu EVN thất bại: "+syncErr.Error())
	}
	usage, err := c.service.Yesterday(ctx)
	if err != nil {
		var unavailable *application.UsageUnavailableError
		if errors.As(err, &unavailable) {
			return c.telegramClient.SendMessage(ctx, chatID,
				"Đã đồng bộ các ngày có số liệu từ EVN.\n"+unavailable.Error())
		}
		return c.telegramClient.SendMessage(ctx, chatID,
			"Đồng bộ dữ liệu EVN thành công. Chưa lấy được báo cáo ngày hôm qua: "+err.Error())
	}
	msg := fmt.Sprintf(
		"Đồng bộ dữ liệu EVN thành công!\nĐiện năng ngày %s:\n Tiêu thụ: %.2f KWh\n Tổng tiền tháng này: %s",
		usage.MeasurementDate.Format("02/01/2006"), usage.ConsumptionKWh,
		utils.FormatVND(usage.TotalAmount),
	)
	return c.telegramClient.SendMessage(ctx, chatID, msg)
}

func (c *ElectricityCommand) sendChunked(ctx context.Context, chatID int64, fullText string) error {
	lines := strings.Split(fullText, "\n")
	var currentChunk strings.Builder

	for _, line := range lines {
		if currentChunk.Len()+len(line)+1 > 4000 {
			if err := c.telegramClient.SendMessage(ctx, chatID, currentChunk.String()); err != nil {
				return err
			}
			currentChunk.Reset()
		}
		currentChunk.WriteString(line)
		currentChunk.WriteString("\n")
	}

	if currentChunk.Len() > 0 {
		return c.telegramClient.SendMessage(ctx, chatID, currentChunk.String())
	}
	return nil
}
