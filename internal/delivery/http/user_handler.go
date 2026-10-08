package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/earnmart/earnmart-be/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserHandler struct {
	service *service.UserService
}

func NewUserHandler(service *service.UserService) *UserHandler {
	return &UserHandler{service: service}
}

type createUserRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=120"`
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type updateUserRequest struct {
	Name  *string `json:"name" binding:"omitempty,min=2,max=120"`
	Email *string `json:"email" binding:"omitempty,email,max=255"`
}

func (h *UserHandler) Create(c *gin.Context) {
	var request createUserRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		failure(c, http.StatusBadRequest, "VALIDATION_ERROR", "Request body is invalid", err.Error())
		return
	}

	user, err := h.service.Create(c.Request.Context(), service.CreateUserInput{
		Name: request.Name, Email: request.Email, Password: request.Password,
	})
	if err != nil {
		domainFailure(c, err)
		return
	}
	success(c, http.StatusCreated, user)
}

func (h *UserHandler) GetByID(c *gin.Context) {
	id, ok := validID(c)
	if !ok {
		return
	}
	user, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		domainFailure(c, err)
		return
	}
	success(c, http.StatusOK, user)
}

func (h *UserHandler) List(c *gin.Context) {
	page, err := positiveIntQuery(c, "page", 1)
	if err != nil {
		failure(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	pageSize, err := positiveIntQuery(c, "page_size", 20)
	if err != nil {
		failure(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	result, err := h.service.List(c.Request.Context(), page, pageSize)
	if err != nil {
		domainFailure(c, err)
		return
	}
	success(c, http.StatusOK, result)
}

func (h *UserHandler) Update(c *gin.Context) {
	id, ok := validID(c)
	if !ok {
		return
	}
	var request updateUserRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		failure(c, http.StatusBadRequest, "VALIDATION_ERROR", "Request body is invalid", err.Error())
		return
	}
	user, err := h.service.Update(c.Request.Context(), id, service.UpdateUserInput{Name: request.Name, Email: request.Email})
	if err != nil {
		domainFailure(c, err)
		return
	}
	success(c, http.StatusOK, user)
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, ok := validID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		domainFailure(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func validID(c *gin.Context) (string, bool) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		failure(c, http.StatusBadRequest, "VALIDATION_ERROR", "id must be a valid UUID", nil)
		return "", false
	}
	return id, true
}

func positiveIntQuery(c *gin.Context, key string, fallback int) (int, error) {
	value := c.Query(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, errors.New(key + " must be a positive integer")
	}
	return parsed, nil
}
