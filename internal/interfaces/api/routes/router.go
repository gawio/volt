package routes

import (
	"database/sql"
	"net/http"

	"github.com/gamee1910/volt/internal/infrastructure/di"
	"github.com/gin-gonic/gin"
)

func SetupRouter(db *sql.DB, container *di.Container) http.Handler {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		if err := db.PingContext(c.Request.Context()); err != nil {
			c.String(http.StatusServiceUnavailable, "database unavailable")
			return
		}
		c.String(http.StatusOK, "OK")
	})

	electricity := container.ElectricityHandler()
	v1 := r.Group("/api/v1")
	v1.POST("/sync", electricity.SyncFromEVNHandler)
	v1.GET("", electricity.GetAllHandler)
	v1.GET("/", electricity.GetAllHandler)
	v1.GET("/yesterday", electricity.GetYesterdayUsageHandler)

	return r
}
