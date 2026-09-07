package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/belyaevedu/remote-code-service/internal/domain"
)

func (r *Repository) SaveUser(ctx context.Context, user *domain.User) error {
	// using the UNIQUE constraint on login
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, login, password_hash) VALUES ($1, $2, $3)`,
		user.ID, user.Login, user.Password,
	)
	return mapPgError(err)
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	return r.getUser(ctx, "SELECT id, login, password_hash FROM users WHERE id = $1", id)
}

func (r *Repository) GetUserByLogin(ctx context.Context, login string) (*domain.User, error) {
	return r.getUser(ctx, "SELECT id, login, password_hash FROM users WHERE login = $1", login)
}

func (r *Repository) getUser(ctx context.Context, query, arg string) (*domain.User, error) {
	var (
		id       string
		found    string
		password string
	)

	err := r.pool.QueryRow(ctx, query, arg).Scan(&id, &found, &password)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return &domain.User{
		ID:       id,
		Login:    found,
		Password: password,
	}, nil
}
