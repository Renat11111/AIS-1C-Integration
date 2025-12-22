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
// @Router       /data [post]
// @Router       /data [put]
// @Router       /data [delete]
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

	if req.ID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.APIResponse{
			Success: false,
			Error:   "Missing 'id' field (envelope ID)",
		})
		return
	}

	if (r.Method == http.MethodPut || r.Method == http.MethodDelete) && req.Data.SaleId == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.APIResponse{
			Success: false,
			Error:   "Missing 'data.SaleId' for " + r.Method + " operation",
		})
		return
	}

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
