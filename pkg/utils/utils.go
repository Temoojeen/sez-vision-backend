package utils

import (
	"regexp"
)

// ValidatePassword проверяет соответствие пароля требованиям
func ValidatePassword(password string) (bool, string) {
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
