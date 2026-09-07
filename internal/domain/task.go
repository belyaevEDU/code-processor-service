package domain

import (
	"errors"
)

var (
	ErrTaskNotFound          = errors.New("task not found")
	ErrAccessDenied          = errors.New("access denied")
	ErrUnsupportedTranslator = errors.New("unsupported translator")
	ErrExecutionTimeout      = errors.New("execution timed out")
	ErrInvalidSubmission     = errors.New("invalid submission")
)

type TaskStatus string

const (
	StatusInProgress TaskStatus = "in_progress"
	StatusReady      TaskStatus = "ready"
)

type Result struct {
	Output string `json:"output,omitempty"`
}

type Task struct {
	ID         string
	UserID     string
	Status     TaskStatus
	Translator string
	Result     *Result
}

// pre-creation api payload
type Submission struct {
	Translator string
	Code       string
}

type TaskMessage struct {
	TaskID     string `json:"task_id"`
	Translator string `json:"translator"`
	Code       string `json:"code"`
}

type ExecutionResult struct {
	Output   string
	ExitCode int // -1 when unavailable
	Failed   bool
}
