package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/earnmart/earnmart-be/internal/service"
	"github.com/gin-gonic/gin"
)

type PasswordResetNotifier interface {
	SendPasswordResetOTP(ctx context.Context, recipient, otp string) error
}

type AuthHandler struct {
	service       *service.AuthService
	exposeTestOTP bool
	notifier      PasswordResetNotifier
}

func NewAuthHandler(service *service.AuthService, exposeTestOTP bool, notifier PasswordResetNotifier) *AuthHandler {
	return &AuthHandler{service: service, exposeTestOTP: exposeTestOTP, notifier: notifier}
}

type registerRequest struct {
	Name         string             `json:"name" binding:"required,min=2,max=120"`
	Email        string             `json:"email" binding:"required,email,max=255"`
	Password     string             `json:"password" binding:"required,min=8,max=72"`
	TermsVersion string             `json:"terms_version" binding:"required,max=64"`
	Device       service.DeviceInfo `json:"device"`
}

type loginRequest struct {
	Email    string             `json:"email" binding:"required,email,max=255"`
	Password string             `json:"password" binding:"required,max=72"`
	Device   service.DeviceInfo `json:"device"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}
type googleRequest struct {
	IDToken string             `json:"id_token" binding:"required"`
	Device  service.DeviceInfo `json:"device"`
}
type forgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email,max=255"`
}
type verifyOTPRequest struct {
	Email string `json:"email" binding:"required,email,max=255"`
	OTP   string `json:"otp" binding:"required,len=6"`
}
type resetPasswordRequest struct {
	ResetTicket string `json:"reset_ticket" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=72"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var request registerRequest
	if !bind(c, &request) {
		return
	}
	result, err := h.service.Register(c.Request.Context(), request.Name, request.Email, request.Password, request.TermsVersion, request.Device)
	if err != nil {
		domainFailure(c, err)
		return
	}
	success(c, http.StatusCreated, result)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var request loginRequest
	if !bind(c, &request) {
		return
	}
	result, err := h.service.Login(c.Request.Context(), request.Email, request.Password, request.Device)
	if err != nil {
		domainFailure(c, err)
		return
	}
	success(c, http.StatusOK, result)
}

func (h *AuthHandler) Google(c *gin.Context) {
	var request googleRequest
	if !bind(c, &request) {
		return
	}
	result, err := h.service.GoogleLogin(c.Request.Context(), request.IDToken, request.Device)
	if err != nil {
		domainFailure(c, err)
		return
	}
	success(c, http.StatusOK, result)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var request refreshRequest
	if !bind(c, &request) {
		return
	}
	result, err := h.service.Refresh(c.Request.Context(), request.RefreshToken)
	if err != nil {
		domainFailure(c, err)
		return
	}
	success(c, http.StatusOK, result)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if err := h.service.Logout(c.Request.Context(), c.GetString("auth_user_id")); err != nil {
		domainFailure(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) Me(c *gin.Context) {
	user, err := h.service.GetUser(c.Request.Context(), c.GetString("auth_user_id"))
	if err != nil {
		domainFailure(c, err)
		return
	}
	success(c, http.StatusOK, user)
}

func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var request forgotPasswordRequest
	if !bind(c, &request) {
		return
	}
	otp, err := h.service.ForgotPassword(c.Request.Context(), request.Email)
	if err != nil {
		domainFailure(c, err)
		return
	}
	if otp != "" && h.notifier != nil {
		if err := h.notifier.SendPasswordResetOTP(c.Request.Context(), request.Email, otp); err != nil {
			domainFailure(c, err)
			return
		}
	}
	response := gin.H{"message": "Nếu email tồn tại, mã OTP đã được gửi."}
	if h.exposeTestOTP && otp != "" {
		response["test_otp"] = otp
	}
	success(c, http.StatusAccepted, response)
}

func (h *AuthHandler) VerifyOTP(c *gin.Context) {
	var request verifyOTPRequest
	if !bind(c, &request) {
		return
	}
	ticket, err := h.service.VerifyPasswordResetOTP(c.Request.Context(), request.Email, request.OTP)
	if err != nil {
		domainFailure(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"reset_ticket": ticket})
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var request resetPasswordRequest
	if !bind(c, &request) {
		return
	}
	if err := h.service.ResetPassword(c.Request.Context(), request.ResetTicket, request.NewPassword); err != nil {
		domainFailure(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"message": "Mật khẩu đã được cập nhật."})
}

func bind(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		failure(c, http.StatusBadRequest, "VALIDATION_ERROR", "Request body is invalid", err.Error())
		return false
	}
	return true
}

func bearerToken(value string) string {
	parts := strings.SplitN(value, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
