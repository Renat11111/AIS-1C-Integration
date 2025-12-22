package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// QueueDepth показывает текущее количество задач в статусе 'pending'
	QueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ais_queue_depth",
		Help: "Current number of pending requests in the database queue",
	})

	// WorkerDuration измеряет время обработки одного запроса (отправка в 1С)
	WorkerDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ais_worker_duration_seconds",
		Help:    "Time taken to send data to 1C",
		Buckets: prometheus.DefBuckets, // Стандартные бакеты: .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
	})

	// ProcessedTotal считает общее количество обработанных задач
	ProcessedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ais_processed_total",
		Help: "Total number of processed requests by status",
	}, []string{"status", "method"}) // labels: status=success/error, method=POST/PUT
)
