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
	CollectionQueue   = "integration_queue"
	WorkerCount       = 5
	BatchSize         = 50
	ChannelBuffer     = 100
	MaxRetries        = 10               // Максимум попыток перед перемещением в DLQ (failed)
	BaseRetryInterval = 1 * time.Minute // Начальный интервал повтора
)

type Service struct {
	app     *pocketbase.PocketBase
	cfg     *config.Config
	client  *http.Client
	bufPool sync.Pool
	jobs    chan *core.Record
	notify  chan struct{} // Канал для пробуждения диспетчера
	wg      sync.WaitGroup
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
		jobs:   make(chan *core.Record, ChannelBuffer),
		notify: make(chan struct{}, 1), // Буферизованный канал, чтобы не блокировать Push
	}
}

func (s *Service) EnsureQueueCollection() error {
	col, err := s.app.FindCollectionByNameOrId(CollectionQueue)
	if err != nil {
		log.Info().Msg("Bootstrap: Creating 'integration_queue' collection...")
		col = core.NewBaseCollection(CollectionQueue)
		col.Type = core.CollectionTypeBase
	} else {
		log.Debug().Msg("Bootstrap: Checking 'integration_queue' fields...")
	}

	// Список полей, которые должны быть в коллекции
	requiredFields := []core.Field{
		&core.TextField{Name: "ais_id", Required: true},
		&core.TextField{Name: "method", Required: true},
		&core.JSONField{Name: "payload", Required: true},
		&core.SelectField{Name: "status", MaxSelect: 1, Values: []string{"pending", "processing", "retry", "failed", "error"}},
		&core.TextField{Name: "error_log"},
		&core.NumberField{Name: "retry_count"},
		&core.DateField{Name: "next_attempt"},
		&core.DateField{Name: "created"},
		&core.DateField{Name: "updated"},
	}

	modified := false
	for _, rf := range requiredFields {
		if col.Fields.GetByName(rf.GetName()) == nil {
			log.Info().Str("field", rf.GetName()).Msg("Adding missing field to collection")
			col.Fields.Add(rf)
			modified = true
		}
	}

	// Проверка и добавление индексов
	indexes := []struct {
		name   string
		unique bool
		cols   string
	}{
		{"idx_status", false, "status"},
		{"idx_ais_id", true, "ais_id"},
		{"idx_next_attempt", false, "next_attempt"},
	}

	for _, idx := range indexes {
		exists := false
		for _, existingIdx := range col.Indexes {
			// В PocketBase v0.35 индексы проверяются по строковому представлению или имени
			if existingIdx == idx.name {
				exists = true
				break
			}
		}
		if !exists {
			log.Info().Str("index", idx.name).Msg("Adding missing index to collection")
			col.AddIndex(idx.name, idx.unique, idx.cols, "")
			modified = true
		}
	}

	if modified || col.Id == "" {
		return s.app.Save(col)
	}

	return nil
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
	
	if err := s.app.Save(record); err != nil {
		return err
	}

	// Уведомляем диспетчер о новой записи
	select {
	case s.notify <- struct{}{}:
	default:
		// Канал полон, диспетчер и так проснется
	}

	return nil
}

func (s *Service) StartBackgroundWorker(ctx context.Context) {
	for i := 0; i < s.cfg.WorkerCount; i++ {
		s.wg.Add(1)
		go s.worker(ctx, i)
	}
	log.Info().Int("count", s.cfg.WorkerCount).Msg("Started 1C integration workers")
	
	s.wg.Add(1)
	go s.dispatcher(ctx)
}

// Wait blocks until all background workers are stopped
func (s *Service) Wait() {
	s.wg.Wait()
	log.Info().Msg("All workers stopped gracefully")
}

func (s *Service) dispatcher(ctx context.Context) {
	defer s.wg.Done()
	log.Info().Msg("Started DB Dispatcher (Event-driven)")
	
	// Таймер теперь может быть реже (например, 5 сек), 
	// так как новые записи будят диспетчер через s.notify
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Первичный запуск
	s.fetchAndDispatch()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Dispatcher stopping...")
			return
		case <-s.notify:
			// Проснулись по сигналу о новой записи
			if count := s.fetchAndDispatch(); count >= s.cfg.BatchSize {
				// Если пачка была полной, возможно там есть еще - проверяем сразу
				select {
				case s.notify <- struct{}{}:
				default:
				}
			}
		case <-ticker.C:
			// Проснулись по времени (для повторов)
			s.fetchAndDispatch()
		}
	}
}

func (s *Service) fetchAndDispatch() int {
	// Считаем глубину очереди (ожидающие + повторы)
	totalPending, _ := s.app.CountRecords(CollectionQueue, dbx.NewExp("status = 'pending' OR status = 'retry'"))
	metrics.QueueDepth.Set(float64(totalPending))

	// Фильтр: либо новые, либо те, кому пора сделать повтор
	filter := "status = 'pending' || (status = 'retry' && next_attempt <= @now)"

	records, err := s.app.FindRecordsByFilter(
		CollectionQueue,
		filter,
		"created",
		s.cfg.BatchSize,
		0,
		nil,
	)
	if err != nil {
		log.Error().Err(err).Msg("Dispatcher failed to fetch records")
		return 0
	}

	count := len(records)
	if count == 0 {
		return 0
	}

	log.Debug().Int("count", count).Msg("Dispatcher found tasks")

	for _, record := range records {
		record.Set("status", "processing")
		if err := s.app.Save(record); err != nil {
			log.Error().Err(err).Str("id", record.Id).Msg("Failed to lock record")
			continue
		}
		s.jobs <- record
	}
	return count
}

func (s *Service) worker(ctx context.Context, id int) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			log.Debug().Int("worker_id", id).Msg("Worker stopping...")
			return
		case record := <-s.jobs:
			s.processRecord(ctx, id, record)
		}
	}
}

func (s *Service) processRecord(ctx context.Context, workerID int, record *core.Record) {
	timer := prometheus.NewTimer(metrics.WorkerDuration)
	defer timer.ObserveDuration()

	buf := s.bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer s.bufPool.Put(buf)

	var data models.AISDocument
	
	if err := json.NewEncoder(buf).Encode(record.Get("payload")); err != nil {
		log.Error().Err(err).Str("record_id", record.Id).Msg("Failed to encode payload")
		s.handleError(record, fmt.Errorf("payload encode error: %w", err), true)
		return
	}

	if err := json.NewDecoder(buf).Decode(&data); err != nil {
		log.Error().Err(err).Str("record_id", record.Id).Msg("Failed to decode payload")
		s.handleError(record, fmt.Errorf("payload decode error: %w", err), true)
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
		s.handleError(record, err, false)
		metrics.ProcessedTotal.WithLabelValues("error", req.Method).Inc()
	} else {
		log.Info().Str("sale_id", req.Data.SaleId).Msg("Worker synced success. Deleting.")
		s.app.Delete(record)
		metrics.ProcessedTotal.WithLabelValues("success", req.Method).Inc()
	}
}

func (s *Service) handleError(record *core.Record, err error, fatal bool) {
	record.Set("error_log", err.Error())
	
	if fatal {
		log.Error().Str("id", record.GetString("ais_id")).Msg("Moving record to DLQ due to fatal error")
		record.Set("status", "failed")
		s.app.Save(record)
		return
	}

	retryCount := record.GetInt("retry_count")
	if retryCount >= MaxRetries {
		log.Warn().Str("id", record.GetString("ais_id")).Msg("Max retries reached. Moving to DLQ (failed).")
		record.Set("status", "failed")
	} else {
		newCount := retryCount + 1
		
		// Используем логику backoff: интервал растет экспоненциально
		// 1m, 2m, 4m, 8m, 16m... (до MaxRetries)
		interval := BaseRetryInterval * (1 << (newCount - 1))
		if interval > 24*time.Hour { // Ограничиваем сверху сутками
			interval = 24 * time.Hour
		}
		
		nextTry := time.Now().Add(interval)
		
		log.Info().Int("attempt", newCount).Time("next_try", nextTry).Str("id", record.GetString("ais_id")).Msg("Scheduling retry")
		
		record.Set("status", "retry")
		record.Set("retry_count", newCount)
		record.Set("next_attempt", nextTry)
	}
	s.app.Save(record)
}

func (s *Service) sendToOneC(ctx context.Context, req models.AISRequest) error {
	_ = fmt.Sprintf("%s", req.ID)
	_ = backoff.NewExponentialBackOff()
	time.Sleep(300 * time.Millisecond)
	return nil
}
