package rest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"ais-1c-proxy/internal/config"
	"ais-1c-proxy/internal/middleware"
	"ais-1c-proxy/internal/models"
	"ais-1c-proxy/internal/service/onec"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// setupTestEnv поднимает временный PocketBase для тестов
func setupTestEnv(t *testing.T) (*pocketbase.PocketBase, *onec.Service, string) {
	tempDir, err := os.MkdirTemp("", "pb_test_*")
	if err != nil {
		t.Fatal(err)
	}

	app := pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir: tempDir,
	})

	if err := app.Bootstrap(); err != nil {
		t.Fatal("Failed to bootstrap app:", err)
	}

	cfg := &config.Config{
		AISToken:    "test-token",
		OneCTimeout: 1 * time.Second,
	}
	service := onec.NewService(app, cfg)
	
	if err := service.EnsureQueueCollection(); err != nil {
		t.Fatal("Failed to create collection:", err)
	}

	return app, service, tempDir
}

func TestReceiveData_Success(t *testing.T) {
	app, service, tempDir := setupTestEnv(t)
	defer os.RemoveAll(tempDir) 

	handler := NewHandler(service)
	cfg := &config.Config{AISToken: "test-token"}
	authMw := middleware.AuthMiddleware(cfg)

	reqBody := models.AISRequest{
		ID:     "test-msg-001",
		Type:   "sale",
		Method: "POST",
		Data: models.AISDocument{
			SaleId: "sale-123",
			SaleMainAmount: 1000.50,
		},
	}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/data", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-token")

	rr := httptest.NewRecorder()
	
	authMw(http.HandlerFunc(handler.ReceiveData)).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	record, err := app.FindFirstRecordByFilter(onec.CollectionQueue, "ais_id = {:id}", map[string]interface{}{"id": "test-msg-001"})
	if err != nil {
		t.Fatalf("Record not found in DB: %v", err)
	}

	if record.GetString("status") != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", record.GetString("status"))
	}
	
	payload := record.Get("payload")
	if payload == nil {
		t.Error("Payload is empty in DB")
	}
}

func TestReceiveData_AuthFail(t *testing.T) {
	_, service, tempDir := setupTestEnv(t)
	defer os.RemoveAll(tempDir)

	handler := NewHandler(service)
	cfg := &config.Config{AISToken: "secret-key"}
	authMw := middleware.AuthMiddleware(cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/data", nil)
	req.Header.Set("X-API-Key", "wrong-key")

	rr := httptest.NewRecorder()
	authMw(http.HandlerFunc(handler.ReceiveData)).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %v", status)
	}
}

func TestReceiveData_Duplicate(t *testing.T) {
	app, service, tempDir := setupTestEnv(t)
	defer os.RemoveAll(tempDir)

	handler := NewHandler(service)
	
	col, _ := app.FindCollectionByNameOrId(onec.CollectionQueue)
	rec := core.NewRecord(col)
	rec.Set("ais_id", "duplicate-id")
	rec.Set("status", "pending")
	rec.Set("method", "POST")
	rec.Set("payload", "{}")
	app.Save(rec)

	reqBody := models.AISRequest{ID: "duplicate-id", Data: models.AISDocument{SaleId: "1"}}
	jsonBody, _ := json.Marshal(reqBody)
	
	req := httptest.NewRequest(http.MethodPost, "/api/v1/data", bytes.NewBuffer(jsonBody))
	rr := httptest.NewRecorder()
	
	handler.ReceiveData(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 OK on duplicate, got %v", rr.Code)
	}
	
	total, _ := app.CountRecords(onec.CollectionQueue, nil)
	if total != 1 {
		t.Errorf("Expected 1 record in DB, got %d", total)
	}
}
