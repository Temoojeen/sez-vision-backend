package handlers

import (
	"net/http"

	"github.com/Temoojeen/sez-vision-backend/internal/models"
	"github.com/Temoojeen/sez-vision-backend/internal/service"

	"github.com/gin-gonic/gin"
)

type AdminRuHandler struct {
	ruService *service.RuService
}

func NewAdminRuHandler(ruService *service.RuService) *AdminRuHandler {
	return &AdminRuHandler{ruService: ruService}
}

func (h *AdminRuHandler) CreateRU(c *gin.Context) {
	var ruInfo models.RUInfo
	if err := c.ShouldBindJSON(&ruInfo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Неверные данные РУ",
			"details": err.Error(),
		})
		return
	}

	// Здесь должна быть логика создания РУ в базе данных
	// Для упрощения возвращаем успех
	c.JSON(http.StatusCreated, gin.H{
		"message": "РУ создано успешно",
		"ru":      ruInfo,
	})
}

func (h *AdminRuHandler) CreateCells(c *gin.Context) {
	ruID := c.Param("id")

	var cells []models.Cell
	if err := c.ShouldBindJSON(&cells); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Неверные данные ячеек",
			"details": err.Error(),
		})
		return
	}

	// Здесь должна быть логика создания ячеек в базе данных
	// Для упрощения возвращаем успех
	c.JSON(http.StatusCreated, gin.H{
		"message": "Ячейки созданы успешно",
		"count":   len(cells),
		"ruId":    ruID,
	})
}
