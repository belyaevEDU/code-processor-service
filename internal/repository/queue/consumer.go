package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/rabbitmq/amqp091-go"

	"github.com/belyaevedu/remote-code-service/internal/config"
	"github.com/belyaevedu/remote-code-service/internal/domain"
	"github.com/belyaevedu/remote-code-service/internal/port"
)

type Consumer struct {
	cfg config.QueueConfig
}

var _ port.TaskConsumer = (*Consumer)(nil)

func NewConsumer(cfg config.QueueConfig) *Consumer {
	if cfg.Prefetch <= 0 {
		cfg.Prefetch = 1
	}
	if cfg.ReconnectDelay <= 0 {
		cfg.ReconnectDelay = time.Second
	}
	return &Consumer{cfg: cfg}
}

// draining the task queue until the context stops
func (c *Consumer) Consume(ctx context.Context, handler port.TaskHandler) error {
	for {
		if ctx.Err() != nil {
			return nil
		}

		if err := c.consumeOnce(ctx, handler); err != nil && ctx.Err() == nil {
			log.Printf("queue consumer: %v, reconnecting in %s", err, c.cfg.ReconnectDelay)
		}

		if !waitReconnect(ctx, c.cfg.ReconnectDelay) {
			return nil
		}
	}
}

// runs a single broker session
func (c *Consumer) consumeOnce(ctx context.Context, handler port.TaskHandler) error {
	conn, ch, err := connect(c.cfg)
	if err != nil {
		return err
	}
	defer func() {
		closeCleanup(ch, "channel")
		closeCleanup(conn, "connection")
	}()

	// bounding in-flight messages
	if err := ch.Qos(c.cfg.Prefetch, 0, false); err != nil {
		return fmt.Errorf("queue qos: %w", err)
	}

	deliveries, err := ch.ConsumeWithContext(ctx, c.cfg.Queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("queue consume: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-deliveries:
			if !ok {
				// the broker closed the channel
				return errors.New("queue deliveries closed: connection lost")
			}
			c.handle(ctx, handler, d)
		}
	}
}

// success acknowledges, a handler failure requeues the message once and a message that already failed once is dropped.
// malformed messages are just dropped
func (c *Consumer) handle(ctx context.Context, handler port.TaskHandler, d amqp091.Delivery) {
	var msg domain.TaskMessage
	if err := json.Unmarshal(d.Body, &msg); err != nil {
		log.Printf("queue consumer: dropping malformed task message: %v", err)
		c.nack(d, false)
		return
	}
	if msg.TaskID == "" {
		log.Printf("queue consumer: dropping task message without an id")
		c.nack(d, false)
		return
	}

	if err := handler(ctx, msg); err != nil {
		if d.Redelivered {
			log.Printf("queue consumer: dropping task %s after a repeated failure: %v", msg.TaskID, err)
			c.nack(d, false)
			return
		}
		log.Printf("queue consumer: requeueing task %s after handler failure: %v", msg.TaskID, err)
		c.nack(d, true)
		return
	}

	c.ack(d)
}

func (c *Consumer) ack(d amqp091.Delivery) {
	if err := d.Ack(false); err != nil {
		log.Printf("queue consumer: acknowledging task: %v", err)
	}
}

func (c *Consumer) nack(d amqp091.Delivery, requeue bool) {
	if err := d.Nack(false, requeue); err != nil {
		log.Printf("queue consumer: nacking task (requeue=%t): %v", requeue, err)
	}
}
