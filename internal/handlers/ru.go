package handlers

import (
	"net/http"
	"strconv"

	"github.com/Temoojeen/sez-vision-backend/internal/models"
	"github.com/Temoojeen/sez-vision-backend/internal/service"

	"github.com/gin-gonic/gin"
)

type RuHandler struct {
	ruService *service.RuService
}

func NewRuHandler(ruService *service.RuService) *RuHandler {
	return &RuHandler{ruService: ruService}
}

func (h *RuHandler) GetRu(c *gin.Context) {
	ruID := c.Param("id")

	response, err := h.ruService.GetRuByID(ruID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_found",
			"message": "РУ не найдено",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *RuHandler) UpdateCellStatus(c *gin.Context) {
	ruID := c.Param("id")
	cellIDStr := c.Param("cellId")

	cellID, err := strconv.Atoi(cellIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_cell_id",
			"message": "Неверный ID ячейки",
		})
		return
	}

	var req models.UpdateCellStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Неверные данные запроса",
			"details": err.Error(),
		})
		return
	}

	cell, err := h.ruService.UpdateCellStatus(ruID, cellID, &req)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "cell not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{
			"error":   "update_error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, cell)
}

func (h *RuHandler) UpdateCellInfo(c *gin.Context) {
	ruID := c.Param("id")
	cellIDStr := c.Param("cellId")

	cellID, err := strconv.Atoi(cellIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_cell_id",
			"message": "Неверный ID ячейки",
		})
		return
	}

	var req models.UpdateCellInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Неверные данные запроса",
			"details": err.Error(),
		})
		return
	}

	cell, err := h.ruService.UpdateCellInfo(ruID, cellID, &req)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "cell not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{
			"error":   "update_error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, cell)
}

func (h *RuHandler) GetHistory(c *gin.Context) {
	ruID := c.Param("id")

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	records, err := h.ruService.GetHistoryByRuID(ruID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Ошибка получения истории",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, records)
}

func (h *RuHandler) UpdateRuStatus(c *gin.Context) {
	ruID := c.Param("id")

	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Неверные данные запроса",
			"details": err.Error(),
		})
		return
	}

	ru, err := h.ruService.UpdateRuStatus(ruID, req.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "update_error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ru)
}

func (h *RuHandler) AddHistory(c *gin.Context) {
	ruID := c.Param("id")

	var req models.AddHistoryRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Неверные данные запроса",
			"details": err.Error(),
		})
		return
	}

	record, err := h.ruService.AddHistoryRecord(ruID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Ошибка добавления записи в историю",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, record)
}

func (h *RuHandler) GetAllRUs(c *gin.Context) {
	rus, err := h.ruService.GetAllRUs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Ошибка получения списка РУ",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, rus)
}

func (h *RuHandler) GetSubstationPublic(c *gin.Context) {
	substationID := c.Param("id")

	rus, err := h.ruService.GetAllRUs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Ошибка получения данных подстанции",
		})
		return
	}

	var filteredRUs []models.RUInfo
	for _, ru := range rus {
		if ru.SubstationID == substationID {
			filteredRUs = append(filteredRUs, ru)
		}
	}

	// Базовые данные подстанции
	substationInfo := gin.H{
		"id":             substationID,
		"name":           getSubstationName(substationID),
		"location":       getSubstationLocation(substationID),
		"description":    getSubstationDescription(substationID),
		"voltage":        getSubstationVoltage(),
		"installedPower": getSubstationPower(),
		"totalRUs":       len(filteredRUs),
		"status":         "operational",
		"rus":            filteredRUs,
	}

	c.JSON(http.StatusOK, gin.H{
		"substation": substationInfo,
	})
}

// Вспомогательные функции (без параметра id)
func getSubstationName(id string) string {
	switch id {
	case "ps-164":
		return "ПС-164"
	case "ps-64":
		return "ПС-64"
	default:
		return "Подстанция " + id
	}
}

func getSubstationLocation(id string) string {
	// Можно дифференцировать по ID если нужно
	switch id {
	case "ps-164":
		return "Северная промзона Хоргос"
	case "ps-64":
		return "Южная промзона Хоргос"
	default:
		return "Промзона Хоргос"
	}
}

func getSubstationDescription(id string) string {
	switch id {
	case "ps-164":
		return "Главная понизительная подстанция №164. Обслуживает северную часть промзоны."
	case "ps-64":
		return "Резервная понизительная подстанция №64. Обслуживает южную часть промзоны."
	default:
		return "Понизительная подстанция. Обслуживает промзону Хоргос."
	}
}

func getSubstationVoltage() string {
	return "110/10 кВ"
}

func getSubstationPower() string {
	return "2 × 25 МВА"
}

// UpdateSubstationRUs - обновление списка РУ на подстанции
func (h *RuHandler) UpdateSubstationRUs(c *gin.Context) {
	substationID := c.Param("id")

	var req struct {
		RuIDs []string `json:"ruIds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Неверные данные запроса",
			"details": err.Error(),
		})
		return
	}

	// Получаем все РУ
	allRUs, err := h.ruService.GetAllRUs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Ошибка получения РУ",
			"details": err.Error(),
		})
		return
	}

	// Фильтруем РУ по ID и обновляем substationId
	var updatedRUs []models.RUInfo
	for _, ruID := range req.RuIDs {
		// Находим РУ в списке всех РУ
		for _, ru := range allRUs {
			if ru.ID == ruID {
				// Обновляем substationId
				ru.SubstationID = substationID
				// Здесь должна быть логика сохранения в БД
				// Для начала просто добавим в ответ
				updatedRUs = append(updatedRUs, ru)
				break
			}
		}
	}

	// TODO: Добавить логику сохранения изменений в БД через сервис

	c.JSON(http.StatusOK, gin.H{
		"message": "РУ успешно обновлены",
		"count":   len(updatedRUs),
		"rus":     updatedRUs,
	})
}
