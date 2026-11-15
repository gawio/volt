package utils

import (
	"fmt"
	"math"
	"time"
	_ "time/tzdata"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const (
	vietnamTimeZone = "Asia/Ho_Chi_Minh"
	startOfDay      = 0
	firstDay        = 1
	evnDateFormat   = "02/01/2006"
)

func LoadVietnamTimezone() (*time.Location, error) {
	loc, err := time.LoadLocation(vietnamTimeZone)
	if err != nil {
		return nil, fmt.Errorf("load Vietnam timezone: %w", err)
	}
	return loc, nil
}

func GetYesterday(now time.Time, loc *time.Location) time.Time {
	return time.Date(
		now.Year(),
		now.Month(),
		now.Day()-1,
		startOfDay, startOfDay, startOfDay, 0,
		loc,
	)
}
func GetFirstDayOfMonth(now time.Time, loc *time.Location) time.Time {
	return time.Date(
		now.Year(),
		now.Month(),
		firstDay,
		startOfDay,
		startOfDay,
		startOfDay,
		0,
		loc,
	)
}

// electricityTiers is the residential tariff from 01/11/2026 (đồng/kWh, before VAT).
// A limit of 0 means "all remaining kWh".
var electricityTiers = []struct {
	limit float64
	price float64
}{
	{100, 1901}, // Bậc 1: 0–100 kWh
	{100, 2289}, // Bậc 2: 101–200 kWh
	{200, 2894}, // Bậc 3: 201–400 kWh
	{300, 3456}, // Bậc 4: 401–700 kWh
	{0, 3844},   // Bậc 5: 701 kWh trở lên
}

const electricityVATRate = 0.08

// CalculateElectricityBill returns the bill including VAT, rounded to whole đồng
// the way EVN does: subtotal and VAT are each rounded, then summed.
func CalculateElectricityBill(totalKWh float64) float64 {
	var subtotal float64
	remainingKWh := totalKWh
	for _, tier := range electricityTiers {
		if remainingKWh <= 0 {
			break
		}
		if tier.limit == 0 || remainingKWh <= tier.limit {
			subtotal += remainingKWh * tier.price
			break
		}
		subtotal += tier.limit * tier.price
		remainingKWh -= tier.limit
	}
	subtotal = math.Round(subtotal)
	vat := math.Round(subtotal * electricityVATRate)
	return subtotal + vat
}

func FormatVND(amount float64) string {
	p := message.NewPrinter(language.Vietnamese)
	return p.Sprintf("%.0f VND", amount)
}

func FormatEVNDate(t time.Time) string {
	return t.Format(evnDateFormat)
}

func ParseEVNDate(value string, loc *time.Location) (time.Time, error) {
	return time.ParseInLocation(evnDateFormat, value, loc)
}
