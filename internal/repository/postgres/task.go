package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/belyaevedu/remote-code-service/internal/domain"
)

func (r *Repository) SaveTask(ctx context.Context, task *domain.Task) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO tasks (id, user_id, status, translator) VALUES ($1, $2, $3, $4)`,
		task.ID, task.UserID, task.Status, task.Translator,
	)
	return err
}

func (r *Repository) GetTask(ctx context.Context, id string) (*domain.Task, error) {
	var (
		userID     string
		status     domain.TaskStatus
		translator string
		result     []byte
	)

	err := r.pool.QueryRow(ctx,
		`SELECT user_id, status, translator, result FROM tasks WHERE id = $1`, id,
	).Scan(&userID, &status, &translator, &result)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrTaskNotFound
	}
	if err != nil {
		return nil, err
	}

	task := &domain.Task{
		ID:         id,
		UserID:     userID,
		Status:     status,
		Translator: translator,
	}
	if result != nil {
		var res domain.Result
		if err := json.Unmarshal(result, &res); err != nil {
			return nil, err
		}
		task.Result = &res
	}
	return task, nil
}

func (r *Repository) UpdateTaskStatus(ctx context.Context, id string, status domain.TaskStatus) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE tasks SET status = $1 WHERE id = $2`, status, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTaskNotFound
	}
	return nil
}

func (r *Repository) SaveTaskResult(ctx context.Context, id string, result *domain.Result) error {
	if result == nil {
		result = &domain.Result{}
	}

	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	tag, err := r.pool.Exec(ctx,
		`UPDATE tasks
		 SET status = 'ready', result = $1, finished_at = now()
		 WHERE id = $2`,
		data, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTaskNotFound
	}
	return nil
}
