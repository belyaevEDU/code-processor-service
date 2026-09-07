package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/belyaevedu/remote-code-service/internal/domain"
	"github.com/belyaevedu/remote-code-service/internal/port"

	"github.com/go-chi/chi/v5"
)

const (
	messageMissingAuthedUser = "missing authenticated user"
)

type TaskHandlers struct {
	taskSvc port.TaskService
}

func NewTaskHandlers(taskSvc port.TaskService) *TaskHandlers {
	return &TaskHandlers{taskSvc: taskSvc}
}

// POST /task
type taskCreateRequest struct {
	Translator string `json:"translator"`
	Code       string `json:"code"`
}

type taskCreateResponse struct {
	TaskID string `json:"task_id"`
}

// GET /status/{task_id}
type taskStatusResponse struct {
	Status string `json:"status"`
}

// GET /result/{task_id}
type taskResultResponse struct {
	Result domain.Result `json:"result"`
}

// @Summary Create a task
// @Description Creating a task owned by an authenticated user and queuing it for execution
// @Tags task
// @Accept json
// @Produce json
// @Param request body taskCreateRequest true "task submission"
// @Success 201 {object} taskCreateResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /task [post]
func (h *TaskHandlers) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r)
	if !ok {
		unauthorizedResponseHelper(w, messageMissingAuthedUser)
		return
	}

	var req taskCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	submission := domain.Submission{
		Translator: req.Translator,
		Code:       req.Code,
	}

	id, err := h.taskSvc.Submit(r.Context(), userID, submission)
	if err != nil {
		if errors.Is(err, domain.ErrUnsupportedTranslator) || errors.Is(err, domain.ErrInvalidSubmission) {
			WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	WriteJSON(w, http.StatusCreated, taskCreateResponse{TaskID: id})
}

// @Summary Get the status of a certain task
// @Description Getting the status of a task by id. Only the owner of the task can access it
// @Tags task
// @Accept json
// @Produce json
// @Param task_id path string true "task id"
// @Success 200 {object} taskStatusResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /status/{task_id} [get]
func (h *TaskHandlers) Status(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r)
	if !ok {
		unauthorizedResponseHelper(w, messageMissingAuthedUser)
		return
	}

	id := chi.URLParam(r, "task_id")

	status, err := h.taskSvc.Status(r.Context(), userID, id)
	if err != nil {
		writeTaskError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, taskStatusResponse{Status: string(status)})
}

// @Summary Get the result of a certain task
// @Description Getting the result of a task by id. Only the owner of the task can access it
// @Tags task
// @Accept json
// @Produce json
// @Param task_id path string true "task id"
// @Success 200 {object} taskResultResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /result/{task_id} [get]
func (h *TaskHandlers) Result(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r)
	if !ok {
		unauthorizedResponseHelper(w, messageMissingAuthedUser)
		return
	}

	id := chi.URLParam(r, "task_id")

	result, err := h.taskSvc.Result(r.Context(), userID, id)
	if err != nil {
		writeTaskError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, taskResultResponse{Result: *result})
}

func userIDFromContext(r *http.Request) (string, bool) {
	id, ok := r.Context().Value(UserIDKey).(string)
	return id, ok && id != ""
}

// both unknown tasks and tasks owned by another user are reported as 404 with the same body
func writeTaskError(w http.ResponseWriter, err error) {
	if errors.Is(err, domain.ErrTaskNotFound) || errors.Is(err, domain.ErrAccessDenied) {
		WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: domain.ErrTaskNotFound.Error()})
		return
	}
	WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
}
