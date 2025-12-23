package onec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
	
	// Circuit Breaker Config
	CBFailureThreshold = 5                // Сколько ошибок подряд до размыкания
	CBOpenDuration     = 30 * time.Second // Сколько ждать перед попыткой восстановления
)

type CBState int

const (
	StateClosed CBState = iota
	StateOpen
	StateHalfOpen
)

type Service struct {
	app     *pocketbase.PocketBase
	cfg     *config.Config
	client  *http.Client
	bufPool sync.Pool
	jobs    chan []*core.Record // Теперь передаем пачки записей
	notify  chan struct{}        // Канал для пробуждения диспетчера
	wg      sync.WaitGroup

	// Circuit Breaker State
	cbState     CBState
	cbFailures  int
	cbLastRetry time.Time
	cbMu        sync.RWMutex
}

func NewService(app *pocketbase.PocketBase, cfg *config.Config) *Service {
	return &Service{
		app:    app,
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.OneCTimeout},
		bufPool: sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, 4096*10)) // Пул побольше для батчей
			},
		},
		jobs:    make(chan []*core.Record, ChannelBuffer),
		notify:  make(chan struct{}, 1),
		cbState: StateClosed,
	}
}

func (s *Service) getCBState() CBState {
// ... (оставляем без изменений)
 s.cbMu.RLock()
	defer s.cbMu.RUnlock()

	if s.cbState == StateOpen {
		if time.Since(s.cbLastRetry) > CBOpenDuration {
			return StateHalfOpen
		}
	}
	return s.cbState
}

func (s *Service) recordCBSuccess() {
	s.cbMu.Lock()
	defer s.cbMu.Unlock()
	s.cbState = StateClosed
	s.cbFailures = 0
	log.Info().Msg("🛡️ Circuit Breaker: CLOSED (System restored)")
}

func (s *Service) recordCBFailure() {
	s.cbMu.Lock()
	defer s.cbMu.Unlock()
	s.cbFailures++
	if s.cbFailures >= CBFailureThreshold && s.cbState != StateOpen {
		s.cbState = StateOpen
		s.cbLastRetry = time.Now()
		log.Warn().Int("failures", s.cbFailures).Msg("🛡️ Circuit Breaker: OPEN (1C is down, pausing requests)")
	}
}

func (s *Service) EnsureQueueCollection() error {
// ... (оставляем без изменений)
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

	// Блокируем всю пачку сразу
	for _, record := range records {
		record.Set("status", "processing")
		if err := s.app.Save(record); err != nil {
			log.Error().Err(err).Str("id", record.Id).Msg("Failed to lock record")
			continue
		}
	}

	// Отправляем всю пачку в канал как ОДНУ задачу для воркера
	s.jobs <- records
	
	return count
}

func (s *Service) worker(ctx context.Context, id int) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			log.Debug().Int("worker_id", id).Msg("Worker stopping...")
			return
		case records := <-s.jobs:
			s.processBatch(ctx, id, records)
		}
	}
}

func (s *Service) processBatch(ctx context.Context, workerID int, records []*core.Record) {
	if len(records) == 0 {
		return
	}

	timer := prometheus.NewTimer(metrics.WorkerDuration)
	defer timer.ObserveDuration()

	// 1. Проверяем состояние Circuit Breaker
	state := s.getCBState()
	if state == StateOpen {
		log.Debug().Int("batch_size", len(records)).Msg("🛡️ Circuit Breaker is OPEN. Skipping batch.")
		for _, r := range records {
			s.handleError(r, fmt.Errorf("circuit breaker is open"), false)
		}
		return
	}

	// 2. Подготавливаем данные (используем пул буферов)
	buf := s.bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer s.bufPool.Put(buf)

	batchRequests := make([]models.AISRequest, 0, len(records))
	for _, record := range records {
		var payloadData models.AISDocument
		
		// Очищаем буфер для каждой записи в батче
		buf.Reset()
		
		// Кодируем payload из записи в буфер
		if err := json.NewEncoder(buf).Encode(record.Get("payload")); err != nil {
			log.Error().Err(err).Str("record_id", record.Id).Msg("Failed to encode payload in batch")
			s.handleError(record, err, true)
			continue
		}

		// Декодируем из буфера в структуру
		if err := json.NewDecoder(buf).Decode(&payloadData); err != nil {
			log.Error().Err(err).Str("record_id", record.Id).Msg("Failed to decode payload in batch")
			s.handleError(record, err, true)
			continue
		}

		batchRequests = append(batchRequests, models.AISRequest{
			ID:     record.GetString("ais_id"),
			Method: record.GetString("method"),
			Data:   payloadData,
		})
	}

	if len(batchRequests) == 0 {
		return
	}

	// Для логов берем SaleId первой записи
	firstSaleID := fmt.Sprintf("%v", batchRequests[0].Data.SaleId)
	log.Info().Int("worker", workerID).Int("batch_size", len(batchRequests)).Str("first_sale_id", firstSaleID).Msg("🚀 Processing batch")

	// 3. Отправляем ПАКЕТ в 1С
	err := s.sendBatchToOneC(ctx, batchRequests)

	if err != nil {
		log.Error().Err(err).Int("batch_size", len(batchRequests)).Msg("❌ Batch sync failed")
		s.recordCBFailure()
		for _, r := range records {
			s.handleError(r, err, false)
			metrics.ProcessedTotal.WithLabelValues("error", "batch").Inc()
		}
	} else {
		log.Info().Int("batch_size", len(batchRequests)).Msg("✅ Batch sync success")
		s.recordCBSuccess()
		for _, r := range records {
			s.app.Delete(r)
			metrics.ProcessedTotal.WithLabelValues("success", "batch").Inc()
		}
	}
}

func (s *Service) handleError(record *core.Record, err error, fatal bool) {
// ... (оставляем без изменений)
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

// RetryFailedTasks переводит все записи из статуса failed в pending
func (s *Service) RetryFailedTasks() (int, error) {
	records, err := s.app.FindRecordsByFilter(
		CollectionQueue,
		"status = 'failed'",
		"created",
		0, // 0 = все записи
		0,
		nil,
	)
	if err != nil {
		return 0, err
	}

	count := len(records)
	if count == 0 {
		return 0, nil
	}

	for _, record := range records {
		record.Set("status", "pending")
		record.Set("retry_count", 0)
		record.Set("error_log", "")
		record.Set("next_attempt", nil)
		if err := s.app.Save(record); err != nil {
			log.Error().Err(err).Str("id", record.Id).Msg("Failed to reset failed record")
		}
	}

	// Будим диспетчер
	select {
	case s.notify <- struct{}{}:
	default:
	}

	log.Info().Int("count", count).Msg("🔄 DLQ: Resetting failed tasks to pending")
	return count, nil
}

func (s *Service) sendBatchToOneC(ctx context.Context, requests []models.AISRequest) error {
	// В будущем здесь будет использоваться backoff для повторных попыток на уровне HTTP
	_ = backoff.NewExponentialBackOff()

	// Имитируем сетевую задержку на обработку пакета
	time.Sleep(500 * time.Millisecond)
	
	// Тестовая логика: если хотя бы в одном ID есть "fail", весь батч падает
	for _, req := range requests {
		if strings.Contains(req.ID, "fail") {
			return fmt.Errorf("1C Batch Error: request %s failed (Simulated)", req.ID)
		}
	}
	
	return nil
}
