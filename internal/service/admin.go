package service

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/Temoojeen/sez-vision-backend/internal/models"
	"github.com/Temoojeen/sez-vision-backend/internal/repository"
	"github.com/Temoojeen/sez-vision-backend/pkg/utils"
)

type AdminService struct {
	userRepo  *repository.UserRepository
	jwtSecret string
}

func NewAdminService(userRepo *repository.UserRepository, jwtSecret string) *AdminService {
	return &AdminService{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
	}
}

// Валидация пароля
func validatePassword(password string) (bool, string) {
	if len(password) < 6 {
		return false, "Пароль должен содержать минимум 6 символов"
	}

	// Проверка на наличие специального символа
	specialCharRegex := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`)
	if !specialCharRegex.MatchString(password) {
		return false, "Пароль должен содержать хотя бы один специальный символ (!@#$%^&* и т.д.)"
	}

	return true, ""
}

func (s *AdminService) GetAllUsers() ([]models.UserResponse, error) {
	users, err := s.userRepo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	var response []models.UserResponse
	for _, user := range users {
		response = append(response, models.UserResponse{
			ID:        user.ID,
			Name:      user.Name,
			Email:     user.Email,
			Role:      string(user.Role),
			CreatedAt: user.CreatedAt,
		})
	}

	return response, nil
}

func (s *AdminService) CreateUser(req *models.AdminCreateRequest) (*models.UserResponse, error) {
	// Проверяем, существует ли пользователь с таким email
	exists, err := s.userRepo.ExistsByEmail(req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if exists {
		return nil, errors.New("user with this email already exists")
	}

	// Валидация пароля
	if valid, message := validatePassword(req.Password); !valid {
		return nil, errors.New(message)
	}

	// Хешируем пароль
	passwordHash, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Преобразуем строку роли в UserRole
	var userRole models.UserRole
	switch req.Role {
	case "admin":
		userRole = models.RoleAdmin
	case "dispatcher":
		userRole = models.RoleDispatcher
	case "engineer":
		userRole = models.RoleEngineer
	default:
		return nil, errors.New("invalid role")
	}

	// Создаем пользователя
	user := &models.User{
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: passwordHash,
		Role:         userRole,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &models.UserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Role:      string(user.Role),
		CreatedAt: user.CreatedAt,
	}, nil
}

func (s *AdminService) UpdateUser(userID string, req *models.AdminUpdateRequest) (*models.UserResponse, error) {
	// Находим пользователя
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	// Проверяем email на уникальность (если email изменился)
	if req.Email != user.Email {
		exists, err := s.userRepo.ExistsByEmail(req.Email)
		if err != nil {
			return nil, fmt.Errorf("failed to check email: %w", err)
		}
		if exists {
			return nil, errors.New("email already taken by another user")
		}
	}

	// Преобразуем строку роли в UserRole
	var userRole models.UserRole
	switch req.Role {
	case "admin":
		userRole = models.RoleAdmin
	case "dispatcher":
		userRole = models.RoleDispatcher
	case "engineer":
		userRole = models.RoleEngineer
	default:
		return nil, errors.New("invalid role")
	}

	// Обновляем данные
	user.Name = req.Name
	user.Email = req.Email
	user.Role = userRole

	// Сохраняем изменения
	if err := s.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return &models.UserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Role:      string(user.Role),
		CreatedAt: user.CreatedAt,
	}, nil
}

func (s *AdminService) DeleteUser(userID string) error {
	// Находим пользователя
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil {
		return errors.New("user not found")
	}

	// Удаляем пользователя
	if err := s.userRepo.Delete(userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

func (s *AdminService) ChangeUserPassword(userID string, req *models.AdminChangePasswordRequest) error {
	// Находим пользователя
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil {
		return errors.New("user not found")
	}

	// Валидация пароля
	if valid, message := validatePassword(req.NewPassword); !valid {
		return errors.New(message)
	}

	// Хешируем новый пароль
	passwordHash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Обновляем пароль
	user.PasswordHash = passwordHash

	// Сохраняем изменения
	if err := s.userRepo.Update(user); err != nil {
		return fmt.Errorf("failed to update user password: %w", err)
	}

	return nil
}
