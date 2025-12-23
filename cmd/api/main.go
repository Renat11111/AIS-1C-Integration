package main

import (
	"context"
	"fmt"
	"net/http"

	"ais-1c-proxy/internal/config"
	"ais-1c-proxy/internal/middleware"
	"ais-1c-proxy/internal/service/onec"
	"ais-1c-proxy/internal/transport/rest"

	_ "ais-1c-proxy/docs"
	"github.com/natefinch/lumberjack"

	"github.com/mattn/go-colorable"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title           AIS-1C Integration Proxy API
// @version         1.0
// @description     Сервис-шлюз для гарантированной доставки данных из AIS в 1С через персистентную очередь.
// @contact.name    Renat
// @host            localhost:8081
// @BasePath        /api

func main() {
	// 1. КРАСИВОЕ ЛОГИРОВАНИЕ С РОТАЦИЕЙ
	logRotation := &lumberjack.Logger{
		Filename:   "app.log",
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     28,   // days
		Compress:   true, // disabled by default
	}

	// В консоль - красиво и с цветами, в файл - JSON
	consoleWriter := zerolog.ConsoleWriter{
		Out:        colorable.NewColorableStdout(),
		TimeFormat: "15:04:05",
	}

	multi := zerolog.MultiLevelWriter(consoleWriter, logRotation)
	log.Logger = zerolog.New(multi).With().Timestamp().Logger()

	log.Info().Msg("🚀 AIS-1C Proxy initialization started...")

	app := pocketbase.New()
	cfg := config.Load()

	onecService := onec.NewService(app, cfg)

	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Automigrate: true,
	})

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := onecService.EnsureQueueCollection(); err != nil {
			return err
		}

		// Создаем контекст для управления воркерами
		workerCtx, workerCancel := context.WithCancel(context.Background())

		// Запускаем воркеры с этим контекстом
		go onecService.StartBackgroundWorker(workerCtx)

		// Регистрируем хук для Graceful Shutdown
		app.OnTerminate().BindFunc(func(te *core.TerminateEvent) error {
			log.Info().Msg("🛑 Shutdown signal received. Stopping workers...")
			workerCancel()     // Сигнализируем воркерам остановиться
			onecService.Wait() // Ждем их завершения
			log.Info().Msg("✅ All workers stopped. Exiting.")
			return te.Next()
		})

		restHandler := rest.NewHandler(onecService)
		legacyAuthMw := middleware.AuthMiddleware(cfg)

		apiHandler := func(evt *core.RequestEvent) error {
			legacyAuthMw(http.HandlerFunc(restHandler.ReceiveData)).ServeHTTP(evt.Response, evt.Request)
			return nil
		}

		retryHandler := func(evt *core.RequestEvent) error {
			legacyAuthMw(http.HandlerFunc(restHandler.RetryFailedTasks)).ServeHTTP(evt.Response, evt.Request)
			return nil
		}

		e.Router.POST("/api/v1/data", apiHandler)
		e.Router.DELETE("/api/v1/data", apiHandler)
		e.Router.POST("/api/v1/queue/retry-failed", retryHandler) // Используем apiHandler так как он содержит Auth + RetryLogic

		e.Router.GET("/api/v1/health", func(evt *core.RequestEvent) error {
			restHandler.HealthCheck(evt.Response, evt.Request)
			return nil
		})

		// Metrics & Swagger
		e.Router.GET("/metrics", func(evt *core.RequestEvent) error {
			promhttp.Handler().ServeHTTP(evt.Response, evt.Request)
			return nil
		})
		e.Router.GET("/swagger", func(evt *core.RequestEvent) error {
			return evt.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
		})
		e.Router.GET("/swagger/{path...}", func(evt *core.RequestEvent) error {
			httpSwagger.WrapHandler(evt.Response, evt.Request)
			return nil
		})

		// ВЫВОД КРАСИВОГО БАННЕРА
		fmt.Println("\n\033[1;32m=====================================================")
		fmt.Printf("  🚀 AIS-1C INTEGRATION SERVICE IS RUNNING\n")
		fmt.Printf("  📦 Version: %s (Commit: %s)\n", config.Version, config.CommitSHA)
		fmt.Println("=====================================================\033[0m")
		fmt.Printf("  \033[1;34m➜ API:\033[0m      http://127.0.0.1:8081/api/v1/data\n")
		fmt.Printf("  \033[1;34m➜ Health:\033[0m   http://127.0.0.1:8081/api/v1/health\n")
		fmt.Printf("  \033[1;34m➜ Admin UI:\033[0m http://127.0.0.1:8081/_/\n")
		fmt.Printf("  \033[1;34m➜ Swagger:\033[0m  http://127.0.0.1:8081/swagger/index.html\n")
		fmt.Printf("  \033[1;34m➜ Metrics:\033[0m  http://127.0.0.1:8081/metrics\n")
		fmt.Printf("  \033[1;34m➜ Grafana:\033[0m  http://localhost:3000 (admin/admin)\n")
		fmt.Printf("  \033[1;34m➜ Prometheus:\033[0m http://localhost:9090\n")
		fmt.Println("\033[1;32m=====================================================\033[0m\n")

		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal().Err(err).Msg("Failed to start application")
	}
}
