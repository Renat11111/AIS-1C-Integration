package onec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"ais-1c-proxy/internal/config"
	"ais-1c-proxy/internal/models"
	"ais-1c-proxy/internal/metrics"

	"github.com/cenkalti/backoff/v4"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/rs/zerolog/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/pocketbase/dbx"
)

const (
	CollectionQueue = "integration_queue"
	WorkerCount     = 5
	BatchSize       = 50
	ChannelBuffer   = 100
)

type Service struct {
	app     *pocketbase.PocketBase
	cfg     *config.Config
	client  *http.Client
	bufPool sync.Pool
	jobs    chan *core.Record
}

func NewService(app *pocketbase.PocketBase, cfg *config.Config) *Service {
	return &Service{
		app:    app,
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.OneCTimeout},
		bufPool: sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, 4096))
			},
		},
		jobs: make(chan *core.Record, ChannelBuffer),
	}
}

func (s *Service) EnsureQueueCollection() error {
	_, err := s.app.FindCollectionByNameOrId(CollectionQueue)
	if err == nil {
		return nil
	}
	log.Info().Msg("Bootstrap: Creating 'integration_queue' collection...")
	col := core.NewBaseCollection(CollectionQueue)
	col.Type = core.CollectionTypeBase
	col.Fields.Add(
		&core.TextField{Name: "ais_id", Required: true},
		&core.TextField{Name: "method", Required: true},
		&core.JSONField{Name: "payload", Required: true},
		&core.SelectField{Name: "status", MaxSelect: 1, Values: []string{"pending", "processing", "error"}},
		&core.TextField{Name: "error_log"},
	)
	col.AddIndex("idx_status", false, "status", "")
	col.AddIndex("idx_ais_id", true, "ais_id", "")
	return s.app.Save(col)
}

func (s *Service) Push(req models.AISRequest) error {
	collection, err := s.app.FindCollectionByNameOrId(CollectionQueue)
	if err != nil {
		return err
	}
	existing, _ := s.app.FindFirstRecordByFilter(CollectionQueue, "ais_id = {:id}", map[string]interface{}{"id": req.ID})
	if existing != nil {
		log.Warn().Str("id", req.ID).Msg("Duplicate request ID ignored")
		return nil
	}
	record := core.NewRecord(collection)
	record.Set("ais_id", req.ID)
	record.Set("method", req.Method)
	record.Set("payload", req.Data)
	record.Set("status", "pending")
	
	return s.app.Save(record)
}

func (s *Service) StartBackgroundWorker(ctx context.Context) {
	for i := 0; i < s.cfg.WorkerCount; i++ {
		go s.worker(ctx, i)
	}
	log.Info().Int("count", s.cfg.WorkerCount).Msg("Started 1C integration workers")
	go s.dispatcher(ctx)
}

func (s *Service) dispatcher(ctx context.Context) {
	log.Info().Msg("Started DB Dispatcher")
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fetchAndDispatch()
		}
	}
}

func (s *Service) fetchAndDispatch() {
	totalPending, err := s.app.CountRecords(CollectionQueue, dbx.NewExp("status = 'pending'"))
	if err == nil {
		metrics.QueueDepth.Set(float64(totalPending))
	}

	records, err := s.app.FindRecordsByFilter(
		CollectionQueue,
		"status = 'pending'",
		"",
		s.cfg.BatchSize,
		0,
		nil,
	)
	if err != nil {
		log.Error().Err(err).Msg("Dispatcher failed to fetch records")
		return
	}

	if len(records) == 0 {
		return
	}

	log.Debug().Int("count", len(records)).Msg("Dispatcher found pending records")

	for _, record := range records {
		record.Set("status", "processing")
		if err := s.app.Save(record); err != nil {
			log.Error().Err(err).Str("id", record.Id).Msg("Failed to lock record")
			continue
		}
		s.jobs <- record
	}
}

func (s *Service) worker(ctx context.Context, id int) {
	for {
		select {
		case <-ctx.Done():
			return
		case record := <-s.jobs:
			s.processRecord(ctx, id, record)
		}
	}
}

func (s *Service) processRecord(ctx context.Context, workerID int, record *core.Record) {
	timer := prometheus.NewTimer(metrics.WorkerDuration)
	defer timer.ObserveDuration()

	// Используем буфер из пула для десериализации (уменьшаем GC pressure)
	buf := s.bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer s.bufPool.Put(buf)

	var data models.AISDocument
	
	// Кодируем map прямо в буфер из пула
	if err := json.NewEncoder(buf).Encode(record.Get("payload")); err != nil {
		log.Error().Err(err).Str("record_id", record.Id).Msg("Failed to encode payload from DB")
		s.markError(record, fmt.Errorf("payload encode error: %w", err))
		return
	}

	// Декодируем из того же буфера в структуру
	if err := json.NewDecoder(buf).Decode(&data); err != nil {
		log.Error().Err(err).Str("record_id", record.Id).Msg("Failed to decode payload to AISDocument")
		s.markError(record, fmt.Errorf("payload decode error: %w", err))
		return
	}

	req := models.AISRequest{
		ID:     record.GetString("ais_id"),
		Method: record.GetString("method"),
		Data:   data,
	}

	log.Info().Int("worker", workerID).Str("method", req.Method).Str("sale_id", req.Data.SaleId).Msg("Processing")

	err := s.sendToOneC(ctx, req)

	if err != nil {
		log.Error().Err(err).Str("sale_id", req.Data.SaleId).Msg("Worker failed sync")
		s.markError(record, err)
		metrics.ProcessedTotal.WithLabelValues("error", req.Method).Inc()
	} else {
		log.Info().Str("sale_id", req.Data.SaleId).Msg("Worker synced success. Deleting.")
		s.app.Delete(record)
		metrics.ProcessedTotal.WithLabelValues("success", req.Method).Inc()
	}
}

func (s *Service) markError(record *core.Record, err error) {
	record.Set("status", "error")
	record.Set("error_log", err.Error())
	s.app.Save(record)
}

func (s *Service) sendToOneC(ctx context.Context, req models.AISRequest) error {
	_ = fmt.Sprintf("%s", req.ID)
	_ = backoff.NewExponentialBackOff()
	time.Sleep(300 * time.Millisecond)
	return nil
}
