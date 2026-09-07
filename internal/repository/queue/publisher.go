package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"

	"github.com/belyaevedu/remote-code-service/internal/config"
	"github.com/belyaevedu/remote-code-service/internal/domain"
	"github.com/belyaevedu/remote-code-service/internal/port"
)

type Publisher struct {
	cfg config.QueueConfig

	conn *amqp091.Connection
	ch   *amqp091.Channel
	mu   sync.Mutex
}

var _ port.TaskPublisher = (*Publisher)(nil)

func NewPublisher(cfg config.QueueConfig) *Publisher {
	if cfg.Prefetch <= 0 {
		cfg.Prefetch = 1
	}
	if cfg.ReconnectDelay <= 0 {
		cfg.ReconnectDelay = time.Second
	}
	return &Publisher{cfg: cfg}
}

func (p *Publisher) Publish(ctx context.Context, msg domain.TaskMessage) error {
	pub, err := newTaskPublishing(msg)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureReady(); err != nil {
		return err
	}

	// "" being the default exchange
	if err := p.ch.PublishWithContext(ctx, "", p.cfg.Queue, false, false, pub); err != nil {
		// the cached connection and/or channel went stale
		// drop, redial and retry the publish exactly once
		if err := p.reset(); err != nil {
			log.Printf("queue publisher: discarding stale session: %v", err)
		}

		if err := p.ensureReady(); err != nil {
			return err
		}

		if err := p.ch.PublishWithContext(ctx, "", p.cfg.Queue, false, false, pub); err != nil {
			if err := p.reset(); err != nil {
				log.Printf("queue publisher: discarding failed session: %v", err)
			}
			return fmt.Errorf("queue publish: %w", err)
		}
	}
	return nil
}

func newTaskPublishing(msg domain.TaskMessage) (amqp091.Publishing, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return amqp091.Publishing{}, fmt.Errorf("queue encode: %w", err)
	}

	return amqp091.Publishing{
		ContentType:   "application/json",
		Body:          body,
		DeliveryMode:  amqp091.Persistent,
		CorrelationId: msg.TaskID,
		Timestamp:     time.Now(),
		Type:          "task.submitted",
	}, nil
}

func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.reset()
}

func (p *Publisher) ensureReady() error {
	if p.ch != nil {
		return nil
	}
	conn, ch, err := connect(p.cfg)
	if err != nil {
		return err
	}
	p.conn, p.ch = conn, ch
	return nil
}

func (p *Publisher) reset() error {
	var errs []error
	if p.ch != nil {
		if err := p.ch.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing channel: %w", err))
		}
	}
	if p.conn != nil {
		if err := p.conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing connection: %w", err))
		}
	}
	p.ch, p.conn = nil, nil
	return errors.Join(errs...)
}
