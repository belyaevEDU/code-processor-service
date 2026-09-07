package port

import (
	"context"

	"github.com/belyaevedu/remote-code-service/internal/domain"
)

type TaskRepository interface {
	SaveTask(ctx context.Context, task *domain.Task) error
	GetTask(ctx context.Context, id string) (*domain.Task, error)
	UpdateTaskStatus(ctx context.Context, id string, status domain.TaskStatus) error
	SaveTaskResult(ctx context.Context, id string, result *domain.Result) error
}

type TaskService interface {
	Submit(ctx context.Context, userID string, sub domain.Submission) (string, error)
	Status(ctx context.Context, userID, id string) (domain.TaskStatus, error)
	Result(ctx context.Context, userID, id string) (*domain.Result, error)
}

type UserRepository interface {
	SaveUser(ctx context.Context, user *domain.User) error
	GetUserByID(ctx context.Context, id string) (*domain.User, error)
	GetUserByLogin(ctx context.Context, login string) (*domain.User, error)
}

type UserService interface {
	Register(ctx context.Context, login, password string) error
	Login(ctx context.Context, login, password string) (string, error)
}

type SessionRepository interface {
	CreateSession(ctx context.Context, session *domain.Session) error
	GetSession(ctx context.Context, sessionID string) (*domain.Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
}

type AuthService interface {
	Authenticate(ctx context.Context, token string) (string, error)
}

type CodeExecutor interface {
	Execute(ctx context.Context, msg domain.TaskMessage) (domain.ExecutionResult, error)
}

type TaskPublisher interface {
	Publish(ctx context.Context, msg domain.TaskMessage) error
}

// processes a single task message consumed from the queue
// returning an error requeues the message once. a message that already failed once is dropped
type TaskHandler func(ctx context.Context, msg domain.TaskMessage) error

type TaskConsumer interface {
	Consume(ctx context.Context, handler TaskHandler) error
}
