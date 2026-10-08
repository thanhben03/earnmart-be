package email

import (
	"context"
	"fmt"
	"net/smtp"

	"github.com/earnmart/earnmart-be/internal/config"
)

type SMTPSender struct{ config config.SMTPConfig }

func NewSMTPSender(cfg config.SMTPConfig) *SMTPSender { return &SMTPSender{config: cfg} }

func (s *SMTPSender) SendPasswordResetOTP(ctx context.Context, recipient, otp string) error {
	if s.config.Host == "" {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	address := s.config.Host + ":" + s.config.Port
	var auth smtp.Auth
	if s.config.Username != "" {
		auth = smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
	}
	from := s.config.From
	if from == "" {
		from = s.config.Username
	}
	message := []byte(fmt.Sprintf("To: %s\r\nFrom: %s\r\nSubject: EarnMart - Ma OTP dat lai mat khau\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nMa OTP cua ban la: %s\r\nMa co hieu luc trong 10 phut.\r\n", recipient, from, otp))
	if err := smtp.SendMail(address, auth, from, []string{recipient}, message); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}
	return nil
}
