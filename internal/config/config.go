package config

import (
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

type Config struct {
	ServerPort    string
	AISToken      string
	OneCBaseURL   string
	OneCUser      string // Логин 1С
	OneCPassword  string // Пароль 1С
	OneCTimeout   time.Duration
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Warn().Msg("No .env file found, using system environment variables")
	}

	return &Config{
		ServerPort:    getEnv("SERVER_PORT", ":8081"),
		AISToken:      getEnv("AIS_TOKEN", ""),
		OneCBaseURL:   getEnv("ONEC_BASE_URL", "http://localhost/base/hs/ais/v1/data"),
		OneCUser:      getEnv("ONEC_USER", ""),
		OneCPassword:  getEnv("ONEC_PASSWORD", ""),
		OneCTimeout:   15 * time.Second,
	}
}

func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}
