package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	SSLMode    string

	ServerPort string
	JWTSecret  string
	JWTTTL     time.Duration
}

func LoadConfig() *Config {
	return &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "service_desk"),
		SSLMode:    getEnv("SSL_MODE", "disable"),

		ServerPort: getEnv("SERVER_PORT", ":8081"),
		JWTSecret:  getEnv("JWT_SECRET", "your-super-secret-jwt-key-change-this-in-production"),
		JWTTTL:     parseDuration(getEnv("JWT_TTL_HOURS", "24")),
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func parseDuration(hoursStr string) time.Duration {
	hours, err := strconv.Atoi(hoursStr)
	if err != nil {
		hours = 24
	}
	return time.Duration(hours) * time.Hour
}
