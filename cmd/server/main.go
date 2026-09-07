package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/belyaevedu/remote-code-service/internal/config"
	"github.com/belyaevedu/remote-code-service/internal/controller"
	"github.com/belyaevedu/remote-code-service/internal/controller/handlers"
	"github.com/belyaevedu/remote-code-service/internal/repository/postgres"
	"github.com/belyaevedu/remote-code-service/internal/repository/queue"
	"github.com/belyaevedu/remote-code-service/internal/repository/redis"
	"github.com/belyaevedu/remote-code-service/internal/service"
)

// @title HTTP REST API for a safe remote code execution service
// @version 1.0
// @description The foundation for a safe remote code execution service
//
// @contact.name Veniamin Belyaev
// @contact.email veniamin@belyaev.work
// @license.name MIT
//
// @BasePath /
//
// @accept json
// @produce json
// @schemes http
//
// @securitydefinitions.bearerauth BearerAuth
func main() {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		log.Fatalf("invalid app config: %v", err)
	}
	dbCfg, err := config.LoadDBConfig()
	if err != nil {
		log.Fatalf("invalid db config: %v", err)
	}
	redisCfg, err := config.LoadRedisConfig()
	if err != nil {
		log.Fatalf("invalid redis config: %v", err)
	}
	queueCfg, err := config.LoadQueueConfig()
	if err != nil {
		log.Fatalf("invalid queue config: %v", err)
	}
	metricsCfg, err := config.LoadMetricsConfig()
	if err != nil {
		log.Fatalf("invalid metrics config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := postgres.Migrate(ctx, dbCfg); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	db, err := postgres.New(ctx, dbCfg)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	sessions, err := redis.New(ctx, redisCfg)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer func() {
		if err := sessions.Close(); err != nil {
			log.Printf("closing redis: %v", err)
		}
	}()

	publisher := queue.NewPublisher(queueCfg)
	defer func() {
		if err := publisher.Close(); err != nil {
			log.Printf("closing queue publisher: %v", err)
		}
	}()

	taskService := service.NewTaskService(db, publisher)
	userService := service.NewUserService(db, sessions)

	taskHandler := handlers.NewTaskHandlers(taskService)
	userHandler := handlers.NewUserHandlers(userService)

	router := controller.NewRouter(taskHandler, userHandler, userService)
	server := controller.NewApi(appCfg.HTTPAddr, router, appCfg.ShutdownTimeout)

	metricsServer := controller.NewMetricsServer(metricsCfg.Addr, metricsCfg.ShutdownTimeout)
	go func() {
		if err := metricsServer.Start(ctx); err != nil {
			log.Fatalf("metrics server exited with error: %v", err)
		}
	}()

	if err := server.Start(ctx); err != nil {
		log.Fatalf("server exited with error: %v", err)
	}
}
