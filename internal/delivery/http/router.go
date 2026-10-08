package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RouterDependencies struct {
	AppName     string
	Environment string
	CORSOrigins []string
	DB          *gorm.DB
	UserHandler *UserHandler
}

func NewRouter(deps RouterDependencies) *gin.Engine {
	if deps.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery(), requestID(), accessLog(), secureHeaders(), cors(deps.CORSOrigins))
	router.NoRoute(func(c *gin.Context) {
		failure(c, http.StatusNotFound, "ROUTE_NOT_FOUND", "Route not found", nil)
	})

	router.GET("/health/live", func(c *gin.Context) {
		success(c, http.StatusOK, gin.H{"status": "ok", "service": deps.AppName, "time": time.Now().UTC()})
	})
	router.GET("/health/ready", func(c *gin.Context) {
		sqlDB, err := deps.DB.DB()
		if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
			failure(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Database is unavailable", nil)
			return
		}
		success(c, http.StatusOK, gin.H{"status": "ready"})
	})

	v1 := router.Group("/api/v1")
	users := v1.Group("/users")
	{
		users.POST("", deps.UserHandler.Create)
		users.GET("", deps.UserHandler.List)
		users.GET("/:id", deps.UserHandler.GetByID)
		users.PATCH("/:id", deps.UserHandler.Update)
		users.DELETE("/:id", deps.UserHandler.Delete)
	}

	return router
}
