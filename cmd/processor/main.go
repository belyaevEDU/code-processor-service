package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/belyaevedu/remote-code-service/internal/config"
	"github.com/belyaevedu/remote-code-service/internal/controller"
	"github.com/belyaevedu/remote-code-service/internal/domain"
	"github.com/belyaevedu/remote-code-service/internal/repository/postgres"
	"github.com/belyaevedu/remote-code-service/internal/repository/queue"
	"github.com/belyaevedu/remote-code-service/internal/service"
)

func main() {
	dbCfg, err := config.LoadDBConfig()
	if err != nil {
		log.Fatalf("invalid db config: %v", err)
	}
	queueCfg, err := config.LoadQueueConfig()
	if err != nil {
		log.Fatalf("invalid queue config: %v", err)
	}
	philCfg, err := config.LoadPhilharmonicConfig()
	if err != nil {
		log.Fatalf("invalid philharmonic config: %v", err)
	}
	metricsCfg, err := config.LoadMetricsConfig()
	if err != nil {
		log.Fatalf("invalid metrics config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := postgres.New(ctx, dbCfg)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	if err := db.WaitReady(ctx); err != nil {
		log.Fatalf("waiting for schema: %v", err)
	}

	phrmExecutor := service.NewPhilharmonicExecutor(philCfg)

	// pulling the sandbox image on every worker before starting to process
	prewarmCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	if err := phrmExecutor.PreWarm(prewarmCtx); err != nil {
		log.Printf("processor: sandbox pre-warm failed: %v (continuing)", err)
	} else {
		log.Printf("processor: sandbox image %q pre-warmed on workers", philCfg.SandboxImage)
	}
	cancel()

	executor := service.NewInstrumentedExecutor(phrmExecutor)

	consumer := queue.NewConsumer(queueCfg)

	metricsServer := controller.NewMetricsServer(metricsCfg.Addr, metricsCfg.ShutdownTimeout)
	go func() {
		if err := metricsServer.Start(ctx); err != nil {
			log.Fatalf("metrics server exited with error: %v", err)
		}
	}()

	handle := func(ctx context.Context, msg domain.TaskMessage) error {
		result, err := executor.Execute(ctx, msg)
		if err != nil {
			return fmt.Errorf("execute task %s: %w", msg.TaskID, err)
		}

		return db.SaveTaskResult(ctx, msg.TaskID, &domain.Result{Output: result.Output})
	}

	log.Printf("processor: consuming queue %q", queueCfg.Queue)
	if err := consumer.Consume(ctx, handle); err != nil {
		log.Fatalf("consumer exited with error: %v", err)
	}
	log.Printf("processor stopped gracefully")
}
