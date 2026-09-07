package redis

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/belyaevedu/remote-code-service/internal/config"
	"github.com/belyaevedu/remote-code-service/internal/domain"
	"github.com/belyaevedu/remote-code-service/internal/port"
)

const sessionKeyPrefix = "session:"

type Repository struct {
	client *redis.Client
	ttl    time.Duration
}

var _ port.SessionRepository = (*Repository)(nil)

func New(ctx context.Context, cfg config.RedisConfig) (*Repository, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		if err := client.Close(); err != nil {
			log.Printf("error raised closing redis client: %v\n", err)
		}
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &Repository{client: client, ttl: cfg.SessionTTL}, nil
}

func (r *Repository) Close() error {
	return r.client.Close()
}

func (r *Repository) CreateSession(ctx context.Context, session *domain.Session) error {
	return r.client.Set(ctx,
		sessionKeyPrefix+session.SessionID, session.UserID, r.ttl,
	).Err()
}

func (r *Repository) GetSession(ctx context.Context, sessionID string) (*domain.Session, error) {
	userID, err := r.client.Get(ctx,
		sessionKeyPrefix+sessionID,
	).Result()
	if errors.Is(err, redis.Nil) {
		return nil, domain.ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}

	return &domain.Session{
		UserID:    userID,
		SessionID: sessionID,
	}, nil
}

func (r *Repository) DeleteSession(ctx context.Context, sessionID string) error {
	removed, err := r.client.Del(ctx,
		sessionKeyPrefix+sessionID,
	).Result()
	if err != nil {
		return err
	}
	if removed == 0 {
		return domain.ErrSessionNotFound
	}
	return nil
}
