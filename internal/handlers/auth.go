package handlers

import (
	"net/http"

	"github.com/Temoojeen/sez-vision-backend/internal/models"
	"github.com/Temoojeen/sez-vision-backend/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	resp, err := h.authService.Register(&req)
	if err != nil {
		status := http.StatusInternalServerError
		errorType := "internal_server_error"
		message := "Failed to register user"

		if err.Error() == "user with this email already exists" {
			status = http.StatusConflict
			errorType = "conflict"
			message = "User with this email already exists"
		}

		c.JSON(status, gin.H{
			"error":   errorType,
			"message": message,
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	resp, err := h.authService.Login(&req)
	if err != nil {
		status := http.StatusInternalServerError
		errorType := "internal_server_error"
		message := "Failed to login"

		if err.Error() == "invalid email or password" {
			status = http.StatusUnauthorized
			errorType = "unauthorized"
			message = "Invalid email or password"
		}

		c.JSON(status, gin.H{
			"error":   errorType,
			"message": message,
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) GetMe(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "User not authenticated",
		})
		return
	}

	resp, err := h.authService.GetCurrentUser(userID.(string))
	if err != nil {
		status := http.StatusInternalServerError
		errorType := "internal_server_error"
		message := "Failed to get user data"

		if err.Error() == "user not found" {
			status = http.StatusNotFound
			errorType = "not_found"
			message = "User not found"
		}

		c.JSON(status, gin.H{
			"error":   errorType,
			"message": message,
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": resp, // Возвращаем как {"user": {...}}
	})
}
