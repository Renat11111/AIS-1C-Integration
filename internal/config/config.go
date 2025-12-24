package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

// Global Build Info
var (
	Version   = "dev"
	CommitSHA = "none"
	BuildTime = "unknown"
)

type Config struct {
	ServerHost   string
	ServerPort   string
	AISToken     string
	OneCBaseURL  string
	OneCUser     string // Логин 1С
	OneCPassword string // Пароль 1С
	OneCTimeout  time.Duration
	WorkerCount  int
	BatchSize    int
}

func Load() *Config {
	// Debug info
	cwd, _ := os.Getwd()
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	log.Debug().Str("cwd", cwd).Str("exe_dir", exeDir).Msg("Config loading started")

	// 1. Попытка загрузить .env из текущей директории
	if err := godotenv.Load(); err != nil {
		log.Warn().Err(err).Msg("Failed to load .env from CWD")

		// 2. Если не вышло, пробуем найти .env рядом с исполняемым файлом (exe)
		envPath := filepath.Join(exeDir, ".env")
		if err := godotenv.Load(envPath); err != nil {
			log.Warn().Err(err).Str("path", envPath).Msg("Failed to load .env from exe dir")
			log.Warn().Msg("Using system environment variables")
		} else {
			log.Info().Str("path", envPath).Msg("Loaded .env from executable directory")
		}
	} else {
		log.Info().Msg("Loaded .env from current directory")
	}

	cfg := &Config{
		ServerHost:   getEnv("SERVER_HOST", "127.0.0.1"),
		ServerPort:   getEnv("SERVER_PORT", "8081"),
		AISToken:     getEnv("AIS_TOKEN", ""),
		OneCBaseURL:  getEnv("ONEC_BASE_URL", "http://localhost/base/hs/ais/v1/data"),
		OneCUser:     getEnv("ONEC_USER", ""),
		OneCPassword: getEnv("ONEC_PASSWORD", ""),
		OneCTimeout:  15 * time.Second,
		WorkerCount:  getEnvAsInt("WORKER_COUNT", 5),
		BatchSize:    getEnvAsInt("BATCH_SIZE", 50),
	}

	// Log loaded token (masked) to verify
	maskedToken := ""
	if len(cfg.AISToken) > 4 {
		maskedToken = cfg.AISToken[:4] + "***"
	} else {
		maskedToken = "***"
	}
	log.Info().Str("token_prefix", maskedToken).Msg("Configuration loaded")

	return cfg
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
