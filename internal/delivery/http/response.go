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
	default:
		failure(c, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred", nil)
	}
}
