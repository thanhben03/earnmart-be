package domain

import "errors"

var (
	ErrNotFound           = errors.New("resource not found")
	ErrAlreadyExists      = errors.New("resource already exists")
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountLocked      = errors.New("account locked")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrSessionExpired     = errors.New("session expired")
	ErrSessionRevoked     = errors.New("session revoked")
	ErrTokenReuse         = errors.New("refresh token reuse detected")
	ErrOTPInvalid         = errors.New("otp invalid")
	ErrOTPExpired         = errors.New("otp expired")
	ErrOTPAttempts        = errors.New("otp attempts exceeded")
	ErrResetTicketInvalid = errors.New("reset ticket invalid")
)
