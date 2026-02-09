package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/Temoojeen/sez-vision-backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *models.User) error {
	// Генерируем UUID если он не установлен
	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	// Устанавливаем временные метки
	now := time.Now()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	if user.UpdatedAt.IsZero() {
		user.UpdatedAt = now
	}

	result := r.db.Create(user)
	if result.Error != nil {
		return fmt.Errorf("failed to create user: %w", result.Error)
	}
	return nil
}

func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	var user models.User
	result := r.db.Where("email = ?", email).First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, fmt.Errorf("failed to find user by email: %w", result.Error)
	}
	return &user, nil
}

func (r *UserRepository) FindByID(id string) (*models.User, error) {
	var user models.User
	result := r.db.Where("id = ?", id).First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, fmt.Errorf("failed to find user by id: %w", result.Error)
	}
	return &user, nil
}

func (r *UserRepository) Update(user *models.User) error {
	// Обновляем UpdatedAt
	user.UpdatedAt = time.Now()

	result := r.db.Save(user)
	if result.Error != nil {
		return fmt.Errorf("failed to update user: %w", result.Error)
	}
	return nil
}

func (r *UserRepository) Delete(id string) error {
	result := r.db.Delete(&models.User{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete user: %w", result.Error)
	}
	return nil
}

func (r *UserRepository) ExistsByEmail(email string) (bool, error) {
	var count int64
	result := r.db.Model(&models.User{}).Where("email = ?", email).Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("failed to check email existence: %w", result.Error)
	}
	return count > 0, nil
}

func (r *UserRepository) GetAll() ([]*models.User, error) {
	var users []*models.User
	result := r.db.Order("created_at DESC").Find(&users)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get all users: %w", result.Error)
	}
	return users, nil
}

func (r *UserRepository) GetUsersByRole(role string) ([]*models.User, error) {
	var users []*models.User
	result := r.db.Where("role = ?", role).Order("created_at DESC").Find(&users)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get users by role: %w", result.Error)
	}
	return users, nil
}

func (r *UserRepository) Count() (int64, error) {
	var count int64
	result := r.db.Model(&models.User{}).Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to count users: %w", result.Error)
	}
	return count, nil
}
