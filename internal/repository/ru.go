package repository

import (
	"fmt"

	"github.com/Temoojeen/sez-vision-backend/internal/models"

	"gorm.io/gorm"
)

type RuRepository struct {
	db *gorm.DB
}

func NewRuRepository(db *gorm.DB) *RuRepository {
	return &RuRepository{db: db}
}

func (r *RuRepository) GetRuByID(ruID string) (*models.RUInfo, error) {
	var ruInfo models.RUInfo
	result := r.db.Where("id = ?", ruID).First(&ruInfo)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get RU by ID: %w", result.Error)
	}
	return &ruInfo, nil
}
func (r *RuRepository) UpdateRu(ruInfo *models.RUInfo) error {
	result := r.db.Save(ruInfo)
	if result.Error != nil {
		return fmt.Errorf("failed to update RU: %w", result.Error)
	}
	return nil
}
func (r *RuRepository) GetCellsByRuID(ruID string) ([]models.Cell, error) {
	var cells []models.Cell
	result := r.db.Where("ru_id = ?", ruID).Order("id ASC").Find(&cells)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get cells by RU ID: %w", result.Error)
	}
	return cells, nil
}

func (r *RuRepository) GetCellByID(cellID int, ruID string) (*models.Cell, error) {
	var cell models.Cell
	result := r.db.Where("id = ? AND ru_id = ?", cellID, ruID).First(&cell)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get cell by ID: %w", result.Error)
	}
	return &cell, nil
}

func (r *RuRepository) UpdateCell(cell *models.Cell) error {
	result := r.db.Save(cell)
	if result.Error != nil {
		return fmt.Errorf("failed to update cell: %w", result.Error)
	}
	return nil
}

func (r *RuRepository) GetHistoryByRuID(ruID string, limit int) ([]models.OperationRecord, error) {
	var records []models.OperationRecord
	query := r.db.Where("ru_id = ?", ruID).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	result := query.Find(&records)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get history by RU ID: %w", result.Error)
	}
	return records, nil
}

func (r *RuRepository) AddHistoryRecord(record *models.OperationRecord) error {
	result := r.db.Create(record)
	if result.Error != nil {
		return fmt.Errorf("failed to add history record: %w", result.Error)
	}
	return nil
}

func (r *RuRepository) GetAllRUs() ([]models.RUInfo, error) {
	var rus []models.RUInfo
	result := r.db.Order("created_at DESC").Find(&rus)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get all RUs: %w", result.Error)
	}
	return rus, nil
}
