package main

import (
	"context"
	"log"
	"net/http"

	"ais-1c-proxy/internal/config"
	"ais-1c-proxy/internal/middleware"
	"ais-1c-proxy/internal/service/onec"
	"ais-1c-proxy/internal/transport/rest"
	
	_ "ais-1c-proxy/docs" // Import generated docs

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title           AIS-1C Proxy API
// @version         1.0
// @description     Сервис интеграции AIS и 1С. Гарантированная доставка сообщений через очередь.
// @termsOfService  http://swagger.io/terms/

// @contact.name    API Support
// @contact.email   support@example.com

// @host            localhost:8081
// @BasePath        /api/v1

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
func main() {
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

		go onecService.StartBackgroundWorker(context.Background())

		restHandler := rest.NewHandler(onecService)
		legacyAuthMw := middleware.AuthMiddleware(cfg)

		apiHandler := func(evt *core.RequestEvent) error {
			finalHandler := legacyAuthMw(http.HandlerFunc(restHandler.ReceiveData))
			finalHandler.ServeHTTP(evt.Response, evt.Request)
			return nil
		}

		e.Router.POST("/api/v1/data", apiHandler)
		e.Router.PUT("/api/v1/data", apiHandler)
		e.Router.DELETE("/api/v1/data", apiHandler)

		// Metrics
		promHandler := promhttp.Handler()
		e.Router.GET("/metrics", func(evt *core.RequestEvent) error {
			promHandler.ServeHTTP(evt.Response, evt.Request)
			return nil
		})

		// Swagger UI
		// Маршрут: /swagger/*
		e.Router.GET("/swagger/*", func(evt *core.RequestEvent) error {
			httpSwagger.WrapHandler(evt.Response, evt.Request)
			return nil
		})

		log.Println("✅ AIS-1C Proxy via PocketBase Started")
		log.Printf("Listening on %s", cfg.ServerPort)
		log.Printf("Swagger UI: http://localhost:8081/swagger/index.html")
		
		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
