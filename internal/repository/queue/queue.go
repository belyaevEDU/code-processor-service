package queue

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/rabbitmq/amqp091-go"

	"github.com/belyaevedu/remote-code-service/internal/config"
)

func closeCleanup(c io.Closer, what string) {
	if err := c.Close(); err != nil {
		log.Printf("queue: closing %s during cleanup: %v", what, err)
	}
}

func connect(cfg config.QueueConfig) (*amqp091.Connection, *amqp091.Channel, error) {
	conn, err := amqp091.Dial(cfg.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("queue dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		closeCleanup(conn, "connection")
		return nil, nil, fmt.Errorf("queue channel: %w", err)
	}

	// durable, autoDelete, exclusive, noWait, args
	if _, err := ch.QueueDeclare(cfg.Queue, true, false, false, false, nil); err != nil {
		closeCleanup(ch, "channel")
		closeCleanup(conn, "connection")
		return nil, nil, fmt.Errorf("queue declare: %w", err)
	}

	return conn, ch, nil
}

// pauses for d, reporting false as soon as ctx is done
func waitReconnect(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}

	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
