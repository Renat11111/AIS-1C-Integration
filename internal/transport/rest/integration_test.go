package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func setupIntegration(t *testing.T) (*pocketbase.PocketBase, *onec.Service, string) {
	tempDir, err := os.MkdirTemp("", "pb_int_*")
	if err != nil {
		t.Fatal(err)
	}

	app := pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir: tempDir,
	})

	if err := app.Bootstrap(); err != nil {
		t.Fatal("Failed to bootstrap:", err)
	}

	cfg := &config.Config{
		AISToken:    "test-secret",
		OneCTimeout: 500 * time.Millisecond,
		WorkerCount: 1,
		BatchSize:   1,
	}
	service := onec.NewService(app, cfg)

	if err := service.EnsureQueueCollection(); err != nil {
		t.Fatal("Failed to create collection:", err)
	}

	return app, service, tempDir
}

func TestIntegration_FullFlow(t *testing.T) {
	app, service, tempDir := setupIntegration(t)
	defer os.RemoveAll(tempDir)

	handler := NewHandler(service)
	authMw := middleware.AuthMiddleware(&config.Config{AISToken: "test-secret"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go service.StartBackgroundWorker(ctx)

	for i := 1; i <= 3; i++ {
		reqBody := models.AISRequest{
			ID:     fmt.Sprintf("msg-%d", i),
			Method: "POST",
			Data:   models.AISDocument{SaleId: fmt.Sprintf("S-%d", i)},
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/data", bytes.NewBuffer(body))
		req.Header.Set("X-API-Key", "test-secret")
		rr := httptest.NewRecorder()
		authMw(http.HandlerFunc(handler.ReceiveData)).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("API request failed: %v", rr.Body.String())
		}
	}

	time.Sleep(2 * time.Second)

	count, _ := app.CountRecords(onec.CollectionQueue, nil)
	if count != 0 {
		t.Errorf("Expected 0 records after processing, but found %d", count)
	}
}

func TestIntegration_CircuitBreaker(t *testing.T) {
	app, service, tempDir := setupIntegration(t)
	defer os.RemoveAll(tempDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go service.StartBackgroundWorker(ctx)

	// Отправляем 6 запросов, которые ГАРАНТИРОВАННО вызывают ошибку
	for i := 0; i < 6; i++ {
		reqBody := models.AISRequest{
			ID:     fmt.Sprintf("fail-%d", i),
			Method: "POST",
			Data:   models.AISDocument{SaleId: "ERR"},
		}
		// Используем Push, чтобы срабатывал сигнал notify
		if err := service.Push(reqBody); err != nil {
			t.Fatalf("Push failed: %v", err)
		}
		time.Sleep(600 * time.Millisecond) // Ждем завершения обработки каждого батча
	}

	// Теперь CB должен быть OPEN. Отправляем нормальный запрос.
	normalReq := models.AISRequest{
		ID:     "normal-one",
		Method: "POST",
		Data:   models.AISDocument{SaleId: "OK"},
	}

	if err := service.Push(normalReq); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	time.Sleep(1 * time.Second)

	updated, err := app.FindFirstRecordByFilter(onec.CollectionQueue, "ais_id = 'normal-one'")
	if err != nil || updated == nil {
		t.Fatal("Normal record not found in DB")
	}

	// Если CB разомкнут, задача должна сразу уйти в 'retry' с логом 'circuit breaker is open'
	if updated.GetString("status") != "retry" {
		t.Errorf("Expected status 'retry' due to CB Open, got '%s'. ErrorLog: %s",
			updated.GetString("status"), updated.GetString("error_log"))
	}

	if updated.GetString("error_log") != "circuit breaker is open" {
		t.Errorf("Expected error log 'circuit breaker is open', got '%s'", updated.GetString("error_log"))
	}
}

func TestIntegration_RetryFailed(t *testing.T) {
	app, service, tempDir := setupIntegration(t)
	defer os.RemoveAll(tempDir)

	handler := NewHandler(service)

	col, _ := app.FindCollectionByNameOrId(onec.CollectionQueue)
	record := core.NewRecord(col)
	record.Set("ais_id", "failed-task")
	record.Set("status", "failed")
	record.Set("payload", map[string]string{"SaleId": "F"})
	record.Set("method", "POST")
	app.Save(record)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/queue/retry-failed", nil)
	rr := httptest.NewRecorder()
	handler.RetryFailedTasks(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Retry endpoint failed: %v", rr.Body.String())
	}

	updated, err := app.FindFirstRecordByFilter(onec.CollectionQueue, "ais_id = 'failed-task'")
	if err != nil || updated == nil {
		t.Fatal("Task not found after retry")
	}

	if updated.GetString("status") != "pending" {
		t.Errorf("Expected status 'pending' after retry, got '%s'", updated.GetString("status"))
	}
}
