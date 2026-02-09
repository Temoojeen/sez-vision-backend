package service

import (
	"fmt"
	"time"

	"github.com/Temoojeen/sez-vision-backend/internal/models"
	"github.com/Temoojeen/sez-vision-backend/internal/repository"

	"github.com/google/uuid"
)

type RuService struct {
	ruRepo *repository.RuRepository
}

func NewRuService(ruRepo *repository.RuRepository) *RuService {
	return &RuService{ruRepo: ruRepo}
}

func (s *RuService) GetRuByID(ruID string) (*models.GetRuResponse, error) {
	ruInfo, err := s.ruRepo.GetRuByID(ruID)
	if err != nil {
		return nil, fmt.Errorf("failed to get RU info: %w", err)
	}

	cells, err := s.ruRepo.GetCellsByRuID(ruID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cells: %w", err)
	}

	return &models.GetRuResponse{
		RuInfo: *ruInfo,
		Cells:  cells,
	}, nil
}

func (s *RuService) UpdateCellStatus(ruID string, cellID int, req *models.UpdateCellStatusRequest) (*models.Cell, error) {
	cell, err := s.ruRepo.GetCellByID(cellID, ruID)
	if err != nil {
		return nil, fmt.Errorf("cell not found: %w", err)
	}

	cell.Status = req.Status
	if req.IsGrounded != nil {
		cell.IsGrounded = *req.IsGrounded
		now := time.Now().Format("02.01.2006 15:04:05")
		cell.LastGroundedOperation = &now
	}

	now := time.Now().Format("02.01.2006 15:04:05")
	cell.LastOperation = &now
	cell.UpdatedAt = time.Now()

	if err := s.ruRepo.UpdateCell(cell); err != nil {
		return nil, fmt.Errorf("failed to update cell: %w", err)
	}

	return cell, nil
}

func (s *RuService) UpdateCellInfo(ruID string, cellID int, req *models.UpdateCellInfoRequest) (*models.Cell, error) {
	cell, err := s.ruRepo.GetCellByID(cellID, ruID)
	if err != nil {
		return nil, fmt.Errorf("cell not found: %w", err)
	}

	cell.Name = req.Name
	cell.Description = req.Description
	cell.Voltage = req.Voltage
	cell.UpdatedAt = time.Now()

	if err := s.ruRepo.UpdateCell(cell); err != nil {
		return nil, fmt.Errorf("failed to update cell info: %w", err)
	}

	return cell, nil
}

func (s *RuService) GetHistoryByRuID(ruID string, limit int) ([]models.OperationRecord, error) {
	records, err := s.ruRepo.GetHistoryByRuID(ruID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}
	return records, nil
}

func (s *RuService) AddHistoryRecord(ruID string, req *models.AddHistoryRecordRequest) (*models.OperationRecord, error) {
	record := &models.OperationRecord{
		ID:                uuid.New().String(),
		CellNumber:        req.CellNumber,
		CellName:          req.CellName,
		Action:            req.Action,
		Operator:          req.Operator,
		Timestamp:         req.Timestamp,
		Reason:            req.Reason,
		DocumentType:      req.DocumentType,
		OrderNumber:       req.OrderNumber,
		WorkOrderNumber:   req.WorkOrderNumber,
		StartDate:         req.StartDate,
		EndDate:           req.EndDate,
		ResponsiblePerson: req.ResponsiblePerson,
		Comment:           req.Comment,
		Severity:          req.Severity,
		RuID:              ruID,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	if err := s.ruRepo.AddHistoryRecord(record); err != nil {
		return nil, fmt.Errorf("failed to add history record: %w", err)
	}

	return record, nil
}

func (s *RuService) GetAllRUs() ([]models.RUInfo, error) {
	rus, err := s.ruRepo.GetAllRUs()
	if err != nil {
		return nil, fmt.Errorf("failed to get all RUs: %w", err)
	}
	return rus, nil
}
func (s *RuService) UpdateRuStatus(ruID string, status string) (*models.RUInfo, error) {
	// Получаем РУ
	ruInfo, err := s.ruRepo.GetRuByID(ruID)
	if err != nil {
		return nil, fmt.Errorf("failed to get RU: %w", err)
	}

	// Обновляем статус
	ruInfo.Status = status
	ruInfo.UpdatedAt = time.Now()

	// Нужно добавить метод UpdateRu в репозитории
	if err := s.ruRepo.UpdateRu(ruInfo); err != nil {
		return nil, fmt.Errorf("failed to update RU status: %w", err)
	}

	return ruInfo, nil
}

// UpdateRUsSubstation - обновление подстанции для списка РУ
func (s *RuService) UpdateRUsSubstation(ruIDs []string, substationID string) ([]models.RUInfo, error) {
	var updatedRUs []models.RUInfo

	for _, ruID := range ruIDs {
		// Получаем РУ
		ruInfo, err := s.ruRepo.GetRuByID(ruID)
		if err != nil {
			continue // Пропускаем если РУ не найдено
		}

		// Обновляем substationId
		ruInfo.SubstationID = substationID
		ruInfo.UpdatedAt = time.Now()

		// Сохраняем изменения
		if err := s.ruRepo.UpdateRu(ruInfo); err != nil {
			return nil, fmt.Errorf("failed to update RU %s: %w", ruID, err)
		}

		updatedRUs = append(updatedRUs, *ruInfo)
	}

	return updatedRUs, nil
}
