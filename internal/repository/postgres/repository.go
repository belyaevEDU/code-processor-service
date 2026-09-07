package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/belyaevedu/remote-code-service/internal/config"
	"github.com/belyaevedu/remote-code-service/internal/domain"
	"github.com/belyaevedu/remote-code-service/internal/port"
)

const (
	pgUniqueViolation = "23505" // login already taken
)

// stores users and tasks
type Repository struct {
	pool *pgxpool.Pool
}

var (
	_ port.TaskRepository = (*Repository)(nil)
	_ port.UserRepository = (*Repository)(nil)
)

func New(ctx context.Context, cfg config.DBConfig) (*Repository, error) {
	pool, err := pgxpool.New(ctx, cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("postgres pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	return &Repository{pool: pool}, nil
}

func (r *Repository) Close() {
	r.pool.Close()
}

func mapPgError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return domain.ErrUserAlreadyExists
	}
	return err
}

const (
	schemaWaitTimeout  = time.Minute
	schemaWaitInterval = 500 * time.Millisecond
)

// blocks until the schema applied by the server's migrations is visible.
// an error is returned once schemaWaitTimeout lapses
func (r *Repository) WaitReady(ctx context.Context) error {
	deadline := time.Now().Add(schemaWaitTimeout)

	for {
		var tasksPresent bool
		err := r.pool.QueryRow(ctx,
			`SELECT to_regclass('public.tasks') IS NOT NULL`,
		).Scan(&tasksPresent)

		if err == nil && tasksPresent {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("schema not ready after %s: has the server migrated?", schemaWaitTimeout)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(schemaWaitInterval):
		}
	}
}
