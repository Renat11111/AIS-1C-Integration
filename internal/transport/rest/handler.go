package rest

import (
	"ais-1c-proxy/internal/models"
	"ais-1c-proxy/internal/service/onec"
	"encoding/json"
	"net/http"
)

type Handler struct {
	service *onec.Service
}

func NewHandler(service *onec.Service) *Handler {
	return &Handler{service: service}
}

// ... (imports)

// ReceiveData принимает данные от AIS
// @Summary      Прием данных (Sale/Update/Delete)
// ...
func (h *Handler) ReceiveData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.AISRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.APIResponse{
			Success: false,
			Error:   "Invalid JSON: " + err.Error(),
		})
		return
	}

	// Использование централизованной валидации
	if err := req.Validate(r.Method); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Явно устанавливаем метод из HTTP (если клиент прислал не то) или доверяем клиенту?
	// Лучше доверять HTTP методу как источнику истины для роутинга
	req.Method = r.Method 

	if err := h.service.Push(req); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.APIResponse{
			Success: false,
			Error:   "Failed to persist data: " + err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models.APIResponse{
		Success: true,
		Message: "Data saved to persistent queue",
	})
}

// HealthCheck возвращает статус сервиса
// @Summary      Health Check
// @Description  Проверяет доступность сервиса
// @Tags         System
// @Produce      json
// @Success      200  {object}  models.APIResponse
// @Router       /health [get]
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models.APIResponse{
		Success: true,
		Message: "OK",
	})
}
