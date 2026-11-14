package application

import (
	"fmt"
	"time"
)

type UsageUnavailableError struct {
	Date time.Time
}

func (e *UsageUnavailableError) Error() string {
	return fmt.Sprintf("Chưa có dữ liệu điện năng ngày %s. Hệ thống sẽ lấy lại trong lần đồng bộ tiếp theo.", e.Date.Format("02/01/2006"))
}
