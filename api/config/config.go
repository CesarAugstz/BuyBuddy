package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port           string
	JWTSecret      string
	GoogleClientID string
	GeminiAPIKey   string
	Environment    string
	CORSOrigins    []string
	Database       DatabaseConfig
}

type DatabaseConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
	SSLMode  string
}

func (d DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "38763"),
		JWTSecret:      getEnv("JWT_SECRET", "dev-secret-key-change-in-production"),
		GoogleClientID: getEnv("GOOGLE_CLIENT_ID", ""),
		GeminiAPIKey:   getEnv("GEMINI_API_KEY", ""),
		Environment:    getEnv("ENV", "development"),
		CORSOrigins:    parseOrigins(getEnv("CORS_ORIGINS", "*")),
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			Name:     getEnv("DB_NAME", "easybuy_dev"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseOrigins(origins string) []string {
	if origins == "*" {
		return []string{"*"}
	}
	return strings.Split(origins, ",")
}
