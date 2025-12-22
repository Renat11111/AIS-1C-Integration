package config

import (
	"os"
	"strconv"
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
	WorkerCount   int
	BatchSize     int
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
		WorkerCount:   getEnvAsInt("WORKER_COUNT", 5),
		BatchSize:     getEnvAsInt("BATCH_SIZE", 50),
	}
}

func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultVal
}
