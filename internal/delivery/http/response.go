package http

import (
	"errors"
	"net/http"

	"github.com/earnmart/earnmart-be/internal/domain"
	"github.com/gin-gonic/gin"
)

type envelope struct {
	Data  any       `json:"data,omitempty"`
	Error *apiError `json:"error,omitempty"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func success(c *gin.Context, status int, data any) {
	c.JSON(status, envelope{Data: data})
}

func failure(c *gin.Context, status int, code, message string, details any) {
	c.AbortWithStatusJSON(status, envelope{Error: &apiError{Code: code, Message: message, Details: details}})
}

func domainFailure(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		failure(c, http.StatusNotFound, "NOT_FOUND", "Resource not found", nil)
	case errors.Is(err, domain.ErrAlreadyExists):
		failure(c, http.StatusConflict, "ALREADY_EXISTS", "Resource already exists", nil)
	case errors.Is(err, domain.ErrInvalidInput):
		failure(c, http.StatusBadRequest, "INVALID_INPUT", "Input is invalid", nil)
	case errors.Is(err, domain.ErrInvalidCredentials):
		failure(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Email hoặc mật khẩu không đúng", nil)
	case errors.Is(err, domain.ErrAccountLocked):
		failure(c, http.StatusLocked, "ACCOUNT_LOCKED", "Tài khoản đã bị khóa", nil)
	case errors.Is(err, domain.ErrAccountDisabled):
		failure(c, http.StatusForbidden, "ACCOUNT_DISABLED", "Tài khoản đã bị vô hiệu hóa", nil)
	case errors.Is(err, domain.ErrSessionExpired):
		failure(c, http.StatusUnauthorized, "SESSION_EXPIRED", "Phiên đăng nhập đã hết hạn", nil)
	case errors.Is(err, domain.ErrSessionRevoked):
		failure(c, http.StatusUnauthorized, "SESSION_REVOKED", "Phiên đăng nhập đã bị thu hồi", nil)
	case errors.Is(err, domain.ErrTokenReuse):
		failure(c, http.StatusUnauthorized, "TOKEN_REUSE_DETECTED", "Phiên đăng nhập không còn an toàn", nil)
	case errors.Is(err, domain.ErrOTPInvalid):
		failure(c, http.StatusBadRequest, "OTP_INVALID", "Mã OTP không hợp lệ", nil)
	case errors.Is(err, domain.ErrOTPExpired):
		failure(c, http.StatusBadRequest, "OTP_EXPIRED", "Mã OTP đã hết hạn", nil)
	case errors.Is(err, domain.ErrOTPAttempts):
		failure(c, http.StatusTooManyRequests, "OTP_ATTEMPTS_EXCEEDED", "Bạn đã nhập sai OTP quá số lần cho phép", nil)
	case errors.Is(err, domain.ErrResetTicketInvalid):
		failure(c, http.StatusBadRequest, "RESET_TICKET_INVALID", "Yêu cầu đặt lại mật khẩu không hợp lệ hoặc đã hết hạn", nil)
	default:
		failure(c, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred", nil)
	}
}
