package middleware

import (
	"ais-1c-proxy/internal/config"
	"ais-1c-proxy/internal/models"
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

func AuthMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				apiKey = r.Header.Get("Authorization") // Fallback
			}

			// Безопасное сравнение токенов (защита от Timing Attack)
			// subtle.ConstantTimeCompare ожидает []byte
			if subtle.ConstantTimeCompare([]byte(apiKey), []byte(cfg.AISToken)) != 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(models.APIResponse{
					Success: false,
					Error:   "Unauthorized: Invalid API Key",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
