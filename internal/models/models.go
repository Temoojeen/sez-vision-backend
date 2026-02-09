package models

import (
	"time"
)

// ================ USER MODELS ================

type UserRole string

const (
	RoleDispatcher UserRole = "dispatcher"
	RoleEngineer   UserRole = "engineer"
	RoleAdmin      UserRole = "admin"
)

type User struct {
	ID           string    `json:"id" gorm:"primaryKey"`
	Name         string    `json:"name"`
	Email        string    `json:"email" gorm:"uniqueIndex"`
	PasswordHash string    `json:"-" gorm:"column:password_hash"`
	Role         UserRole  `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}

// ================ AUTH MODELS ================

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type AuthResponse struct {
	User  UserResponse `json:"user"`
	Token string       `json:"token"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// ================ ADMIN MODELS ================

type AdminCreateRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role" binding:"required,oneof=admin dispatcher engineer"`
}

type AdminUpdateRequest struct {
	Name  string `json:"name" binding:"required,min=2,max=100"`
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role" binding:"required,oneof=admin dispatcher engineer"`
}

// ================ RU MODELS ================

type RUType string

const (
	TypeKRU RUType = "KRU"
	TypeTP  RUType = "TP"
)

type RUInfo struct {
	ID               string    `json:"id" gorm:"primaryKey"`
	Name             string    `json:"name"`
	Voltage          string    `json:"voltage"`
	Sections         int       `json:"sections"`
	CellsCount       int       `json:"cellsCount"`
	Transformers     int       `json:"transformers"`
	TransformerPower string    `json:"transformerPower"`
	Location         string    `json:"location"`
	InstallationDate string    `json:"installationDate"`
	Manufacturer     string    `json:"manufacturer"`
	LastMaintenance  string    `json:"lastMaintenance"`
	NextMaintenance  string    `json:"nextMaintenance"`
	Status           string    `json:"status"`
	SchemeType       string    `json:"schemeType"`
	TotalLoadHigh    string    `json:"totalLoadHigh"`
	TotalLoadLow     string    `json:"totalLoadLow"`
	TotalPowerHigh   string    `json:"totalPowerHigh"`
	TotalPowerLow    string    `json:"totalPowerLow"`
	MaxCapacityHigh  string    `json:"maxCapacityHigh"`
	MaxCapacityLow   string    `json:"maxCapacityLow"`
	OperationalHours int       `json:"operationalHours"`
	LastInspection   string    `json:"lastInspection"`
	Type             RUType    `json:"type"`
	HasHighSide      bool      `json:"hasHighSide"`
	HasLowSide       bool      `json:"hasLowSide"`
	BusSections      int       `json:"busSections"`
	CellsPerSection  int       `json:"cellsPerSection"`
	SubstationID     string    `json:"substationId"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (RUInfo) TableName() string {
	return "ru_infos"
}

type CellType string

const (
	CellTypeInput       CellType = "INPUT"
	CellTypeSR          CellType = "SR"
	CellTypeSV          CellType = "SV"
	CellTypeTransformer CellType = "TRANSFORMER"
	CellTypeReserve     CellType = "RESERVE"
	CellTypeBus         CellType = "BUS"
	CellTypeLowVoltage  CellType = "LOW_VOLTAGE"
	CellTypeOutput      CellType = "OUTPUT"
	CellTypeProtection           = "PROTECTION"  // Защита
	CellTypeMeasurement          = "MEASUREMENT" // Измерение
)

type CellStatus string

const (
	CellStatusON          CellStatus = "ON"
	CellStatusOFF         CellStatus = "OFF"
	CellStatusReserve     CellStatus = "RESERVE"
	CellStatusError       CellStatus = "ERROR"
	CellStatusMaintenance CellStatus = "MAINTENANCE"
)

type Cell struct {
	ID                    int        `json:"id" gorm:"primaryKey;autoIncrement"`
	Number                string     `json:"number"`
	Name                  string     `json:"name"`
	Type                  CellType   `json:"type"`
	Status                CellStatus `json:"status"`
	Voltage               string     `json:"voltage"`
	VoltageLevel          string     `json:"voltageLevel"`
	Power                 *string    `json:"power,omitempty"`
	Description           string     `json:"description"`
	LastOperation         *string    `json:"lastOperation,omitempty"`
	IsGrounded            bool       `json:"isGrounded"`
	LastGroundedOperation *string    `json:"lastGroundedOperation,omitempty"`
	TransformerNumber     *string    `json:"transformerNumber,omitempty"`
	BusSection            *int       `json:"busSection,omitempty"`
	Current               *float64   `json:"current,omitempty"`
	Temperature           *float64   `json:"temperature,omitempty"`
	Load                  *float64   `json:"load,omitempty"`
	RuID                  string     `json:"ruId" gorm:"index"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

func (Cell) TableName() string {
	return "cells"
}

type OperationRecord struct {
	ID                string    `json:"id" gorm:"primaryKey"`
	CellNumber        string    `json:"cellNumber"`
	CellName          string    `json:"cellName"`
	Action            string    `json:"action"`
	Operator          string    `json:"operator"`
	Timestamp         string    `json:"timestamp"`
	Reason            *string   `json:"reason,omitempty"`
	DocumentType      *string   `json:"documentType,omitempty"`
	OrderNumber       *string   `json:"orderNumber,omitempty"`
	WorkOrderNumber   *string   `json:"workOrderNumber,omitempty"`
	StartDate         *string   `json:"startDate,omitempty"`
	EndDate           *string   `json:"endDate,omitempty"`
	ResponsiblePerson *string   `json:"responsiblePerson,omitempty"`
	Comment           *string   `json:"comment,omitempty"`
	Severity          *string   `json:"severity,omitempty"`
	RuID              string    `json:"ruId" gorm:"index"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// UpdateCellInfoRequest - запрос на обновление информации ячейки
type UpdateCellInfoRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=100"`
	Description string `json:"description" binding:"required,min=1,max=500"`
	Voltage     string `json:"voltage" binding:"required,min=1,max=20"`
}

func (OperationRecord) TableName() string {
	return "operation_records"
}

// ================ API RESPONSE MODELS ================

// GetRuResponse - ответ с данными РУ для API
type GetRuResponse struct {
	RuInfo RUInfo `json:"ruInfo"`
	Cells  []Cell `json:"cells"`
}

// UpdateCellStatusRequest - запрос на обновление статуса ячейки
type UpdateCellStatusRequest struct {
	Status     CellStatus `json:"status"`
	IsGrounded *bool      `json:"isGrounded,omitempty"`
}

// AddHistoryRecordRequest - запрос на добавление записи в историю
type AddHistoryRecordRequest struct {
	CellNumber        string  `json:"cellNumber"`
	CellName          string  `json:"cellName"`
	Action            string  `json:"action"`
	Operator          string  `json:"operator"`
	Timestamp         string  `json:"timestamp"`
	Reason            *string `json:"reason,omitempty"`
	DocumentType      *string `json:"documentType,omitempty"`
	OrderNumber       *string `json:"orderNumber,omitempty"`
	WorkOrderNumber   *string `json:"workOrderNumber,omitempty"`
	StartDate         *string `json:"startDate,omitempty"`
	EndDate           *string `json:"endDate,omitempty"`
	ResponsiblePerson *string `json:"responsiblePerson,omitempty"`
	Comment           *string `json:"comment,omitempty"`
	Severity          *string `json:"severity,omitempty"`
}

// ================ PASSWORD CHANGE MODELS ================

type AdminChangePasswordRequest struct {
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}
