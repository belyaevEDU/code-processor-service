package service

import (
	"context"
	"fmt"

	"github.com/belyaevedu/remote-code-service/internal/domain"
	"github.com/belyaevedu/remote-code-service/internal/port"

	"github.com/google/uuid"
)

type TaskService struct {
	repo      port.TaskRepository
	publisher port.TaskPublisher
}

var _ port.TaskService = (*TaskService)(nil)

func NewTaskService(repo port.TaskRepository, publisher port.TaskPublisher) *TaskService {
	return &TaskService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *TaskService) Submit(ctx context.Context, userID string, sub domain.Submission) (string, error) {
	if userID == "" {
		return "", domain.ErrAccessDenied
	}
	if _, ok := supportedTranslators[sub.Translator]; !ok {
		return "", fmt.Errorf("%w: %q", domain.ErrUnsupportedTranslator, sub.Translator)
	}
	if sub.Code == "" {
		return "", fmt.Errorf("%w: empty code", domain.ErrInvalidSubmission)
	}

	id := uuid.NewString()

	task := &domain.Task{
		ID:         id,
		UserID:     userID,
		Status:     domain.StatusInProgress,
		Translator: sub.Translator,
	}

	if err := s.repo.SaveTask(ctx, task); err != nil {
		return "", err
	}

	msg := domain.TaskMessage{
		TaskID:     id,
		Translator: sub.Translator,
		Code:       sub.Code,
	}
	if err := s.publisher.Publish(ctx, msg); err != nil {
		// the row stays behind as in_progress evidence of the failure
		return "", fmt.Errorf("queue publish: %w", err)
	}

	return id, nil
}

func (s *TaskService) Status(ctx context.Context, userID, id string) (domain.TaskStatus, error) {
	t, err := s.getOwnedTask(ctx, userID, id)
	if err != nil {
		return "", err
	}
	return t.Status, nil
}

func (s *TaskService) Result(ctx context.Context, userID, id string) (*domain.Result, error) {
	t, err := s.getOwnedTask(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if t.Result == nil {
		return nil, domain.ErrTaskNotFound
	}
	return t.Result, nil
}

// fetches the task by id and verifies it belongs to a set user
// task owned by someone else results in ErrAccessDenied
func (s *TaskService) getOwnedTask(ctx context.Context, userID, id string) (*domain.Task, error) {
	if userID == "" {
		return nil, domain.ErrAccessDenied
	}

	t, err := s.repo.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}

	if t.UserID != userID {
		return nil, domain.ErrAccessDenied
	}

	return t, nil
}
