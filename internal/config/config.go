package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App      AppConfig
	HTTP     HTTPConfig
	Database DatabaseConfig
	Security SecurityConfig
	SMTP     SMTPConfig
}

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

type AppConfig struct {
	Name               string
	Environment        string
	MaintenanceMode    bool
	MaintenanceMessage string
	TermsVersion       string
	TermsContent       string
}

type HTTPConfig struct {
	Port             string
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	IdleTimeout      time.Duration
	ShutdownTimeout  time.Duration
	CORSAllowOrigins []string
}

type DatabaseConfig struct {
	Host            string
	Port            string
	Name            string
	User            string
	Password        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=UTC", c.User, c.Password, c.Host, c.Port, c.Name)
}

type SecurityConfig struct {
	BcryptCost      int
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	OTPTokenTTL     time.Duration
	ResetTicketTTL  time.Duration
	OTPPepper       string
	ExposeTestOTP   bool
	GoogleClientIDs []string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	readTimeout, err := durationEnv("HTTP_READ_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	writeTimeout, err := durationEnv("HTTP_WRITE_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	idleTimeout, err := durationEnv("HTTP_IDLE_TIMEOUT", 60*time.Second)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := durationEnv("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	connMaxLifetime, err := durationEnv("DB_CONN_MAX_LIFETIME", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	maxOpenConns, err := intEnv("DB_MAX_OPEN_CONNS", 25)
	if err != nil {
		return Config{}, err
	}
	maxIdleConns, err := intEnv("DB_MAX_IDLE_CONNS", 10)
	if err != nil {
		return Config{}, err
	}
	bcryptCost, err := intEnv("BCRYPT_COST", 12)
	if err != nil {
		return Config{}, err
	}
	accessTokenTTL, err := durationEnv("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	refreshTokenTTL, err := durationEnv("REFRESH_TOKEN_TTL", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	otpTokenTTL, err := durationEnv("OTP_TOKEN_TTL", 10*time.Minute)
	if err != nil {
		return Config{}, err
	}
	resetTicketTTL, err := durationEnv("RESET_TICKET_TTL", 10*time.Minute)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		App: AppConfig{
			Name:               stringEnv("APP_NAME", "earnmart-api"),
			Environment:        stringEnv("APP_ENV", "development"),
			MaintenanceMode:    boolEnv("MAINTENANCE_MODE", false),
			MaintenanceMessage: stringEnv("MAINTENANCE_MESSAGE", "Hệ thống đang bảo trì. Vui lòng thử lại sau."),
			TermsVersion:       stringEnv("TERMS_VERSION", "2026-10-08"),
			TermsContent:       stringEnv("TERMS_CONTENT", "Điều khoản sử dụng EarnMart"),
		},
		HTTP: HTTPConfig{
			Port:             stringEnv("HTTP_PORT", "8080"),
			ReadTimeout:      readTimeout,
			WriteTimeout:     writeTimeout,
			IdleTimeout:      idleTimeout,
			ShutdownTimeout:  shutdownTimeout,
			CORSAllowOrigins: csvEnv("CORS_ALLOW_ORIGINS", "http://localhost:3000"),
		},
		Database: DatabaseConfig{
			Host:            stringEnv("DB_HOST", "localhost"),
			Port:            stringEnv("DB_PORT", "3306"),
			Name:            stringEnv("DB_NAME", "earnmart"),
			User:            stringEnv("DB_USER", "earnmart"),
			Password:        os.Getenv("DB_PASSWORD"),
			MaxOpenConns:    maxOpenConns,
			MaxIdleConns:    maxIdleConns,
			ConnMaxLifetime: connMaxLifetime,
		},
		Security: SecurityConfig{
			BcryptCost:      bcryptCost,
			JWTSecret:       stringEnv("JWT_SECRET", "development-only-change-this-secret-now"),
			AccessTokenTTL:  accessTokenTTL,
			RefreshTokenTTL: refreshTokenTTL,
			OTPTokenTTL:     otpTokenTTL,
			ResetTicketTTL:  resetTicketTTL,
			OTPPepper:       stringEnv("OTP_PEPPER", "development-only-otp-pepper"),
			ExposeTestOTP:   boolEnv("AUTH_EXPOSE_TEST_OTP", false),
			GoogleClientIDs: csvEnv("GOOGLE_CLIENT_IDS", ""),
		},
		SMTP: SMTPConfig{
			Host:     stringEnv("SMTP_HOST", ""),
			Port:     stringEnv("SMTP_PORT", "587"),
			Username: stringEnv("SMTP_USERNAME", ""),
			Password: os.Getenv("SMTP_PASSWORD"),
			From:     stringEnv("SMTP_FROM", ""),
		},
	}

	// if cfg.Database.Password == "" {
	// 	return Config{}, errors.New("DB_PASSWORD is required")
	// }
	if cfg.Security.BcryptCost < 10 || cfg.Security.BcryptCost > 31 {
		return Config{}, errors.New("BCRYPT_COST must be between 10 and 31")
	}
	if cfg.Database.MaxIdleConns > cfg.Database.MaxOpenConns {
		return Config{}, errors.New("DB_MAX_IDLE_CONNS must not exceed DB_MAX_OPEN_CONNS")
	}
	if cfg.App.Environment == "production" {
		if cfg.Database.Password == "" {
			return Config{}, errors.New("DB_PASSWORD is required in production")
		}
		if len(cfg.Security.JWTSecret) < 32 || cfg.Security.JWTSecret == "development-only-change-this-secret-now" {
			return Config{}, errors.New("JWT_SECRET must be a unique value of at least 32 characters in production")
		}
		if len(cfg.Security.OTPPepper) < 32 || cfg.Security.OTPPepper == "development-only-otp-pepper" {
			return Config{}, errors.New("OTP_PEPPER must be a unique value of at least 32 characters in production")
		}
		if cfg.Security.ExposeTestOTP {
			return Config{}, errors.New("AUTH_EXPOSE_TEST_OTP must be false in production")
		}
		if len(cfg.Security.GoogleClientIDs) == 0 {
			return Config{}, errors.New("GOOGLE_CLIENT_IDS is required in production")
		}
		if cfg.SMTP.Host == "" || cfg.SMTP.From == "" {
			return Config{}, errors.New("SMTP_HOST and SMTP_FROM are required in production")
		}
	}

	return cfg, nil
}

func stringEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func intEnv(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}
	return parsed, nil
}

func csvEnv(key, fallback string) []string {
	values := strings.Split(stringEnv(key, fallback), ",")
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func boolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
