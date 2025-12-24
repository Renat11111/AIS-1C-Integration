package rest

import (
	"ais-1c-proxy/internal/config"
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
// @Description  Принимает JSON пакет от AIS, ставит в очередь на отправку в 1С.
// @Tags         Integration
// @Accept       json
// @Produce      json
// @Param        X-API-Key header string true "API Key"
// @Param        body body models.AISRequest true "Пакет данных"
// @Success      200  {object}  models.APIResponse
// @Failure      400  {object}  models.APIResponse
// @Failure      401  {object}  models.APIResponse
// @Failure      500  {object}  models.APIResponse
// @Router       /v1/data [post]
// @Router       /v1/data [delete]
func (h *Handler) ReceiveData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.AISRequest
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&req); err != nil {
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

	// Явно устанавливаем метод из HTTP (источник истины)
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
// @Router       /v1/health [get]
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"message":    "OK",
		"version":    config.Version,
		"commit_sha": config.CommitSHA,
		"build_time": config.BuildTime,
	})
}

// RetryFailedTasks перезапускает задачи со статусом failed
// @Summary      Перезапуск упавших задач
// @Description  Находит все задачи в статусе failed и возвращает их в очередь (статус pending)
// @Tags         System
// @Produce      json
// @Param        X-API-Key header string true "API Key"
// @Success      200  {object}  models.APIResponse
// @Failure      500  {object}  models.APIResponse
// @Router       /v1/queue/retry-failed [post]
func (h *Handler) RetryFailedTasks(w http.ResponseWriter, r *http.Request) {
	count, err := h.service.RetryFailedTasks()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.APIResponse{
			Success: false,
			Error:   "Failed to retry tasks: " + err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models.APIResponse{
		Success: true,
		Message: "Successfully reset tasks",
		Data:    map[string]int{"retried_count": count},
	})
}
