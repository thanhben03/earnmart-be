package http

import (
	"net/http"
	"time"

	"github.com/earnmart/earnmart-be/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RouterDependencies struct {
	AppName            string
	Environment        string
	CORSOrigins        []string
	DB                 *gorm.DB
	UserHandler        *UserHandler
	AuthHandler        *AuthHandler
	AuthService        *service.AuthService
	MaintenanceMode    bool
	MaintenanceMessage string
	TermsVersion       string
	TermsContent       string
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
	v1.GET("/app/bootstrap", func(c *gin.Context) {
		success(c, http.StatusOK, gin.H{"maintenance": deps.MaintenanceMode, "maintenance_message": deps.MaintenanceMessage, "terms_version": deps.TermsVersion})
	})
	v1.GET("/legal/terms/current", func(c *gin.Context) {
		success(c, http.StatusOK, gin.H{"version": deps.TermsVersion, "content": deps.TermsContent})
	})

	v1.Use(maintenanceGate(deps.MaintenanceMode, deps.MaintenanceMessage))
	auth := v1.Group("/auth")
	{
		auth.POST("/register", rateLimit(10, time.Minute), deps.AuthHandler.Register)
		auth.POST("/login", rateLimit(10, time.Minute), deps.AuthHandler.Login)
		auth.POST("/google", rateLimit(10, time.Minute), deps.AuthHandler.Google)
		auth.POST("/refresh", rateLimit(30, time.Minute), deps.AuthHandler.Refresh)
		auth.POST("/password/forgot", rateLimit(5, time.Hour), deps.AuthHandler.ForgotPassword)
		auth.POST("/password/verify-otp", rateLimit(10, 15*time.Minute), deps.AuthHandler.VerifyOTP)
		auth.POST("/password/reset", rateLimit(10, 15*time.Minute), deps.AuthHandler.ResetPassword)
		auth.POST("/logout", authenticate(deps.AuthService), deps.AuthHandler.Logout)
	}
	v1.GET("/me", authenticate(deps.AuthService), deps.AuthHandler.Me)

	users := v1.Group("/users")
	users.Use(authenticate(deps.AuthService))
	{
		users.POST("", deps.UserHandler.Create)
		users.GET("", deps.UserHandler.List)
		users.GET("/:id", deps.UserHandler.GetByID)
		users.PATCH("/:id", deps.UserHandler.Update)
		users.DELETE("/:id", deps.UserHandler.Delete)
	}

	return router
}
