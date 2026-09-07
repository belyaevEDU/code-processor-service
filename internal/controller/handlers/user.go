package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/belyaevedu/remote-code-service/internal/domain"
	"github.com/belyaevedu/remote-code-service/internal/port"
)

type UserHandlers struct {
	userSvc port.UserService
}

func NewUserHandlers(userSvc port.UserService) *UserHandlers {
	return &UserHandlers{userSvc: userSvc}
}

// body for /register and /login
type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// response body for /register
type registerResponse struct {
	Message string `json:"message"`
}

// response body for /login
type loginResponse struct {
	Token string `json:"token"`
}

// @Summary Register a user
// @Description Registering a user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body authRequest true "registration credentials"
// @Success 201 {object} registerResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /register [post]
func (h *UserHandlers) Register(w http.ResponseWriter, r *http.Request) {
	req, err := decodeAuth(r)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.userSvc.Register(r.Context(), req.Username, req.Password); err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		if errors.Is(err, domain.ErrUserAlreadyExists) {
			WriteJSON(w, http.StatusConflict, ErrorResponse{Error: err.Error()})
			return
		}
		log.Printf("error raised in userhandlers' register: %v\n", err)
		WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	WriteJSON(w, http.StatusCreated, registerResponse{Message: "user registered"})
}

// @Summary Login as a user
// @Description Logging in as a user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body authRequest true "login credentials"
// @Success 200 {object} loginResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /login [post]
func (h *UserHandlers) Login(w http.ResponseWriter, r *http.Request) {
	req, err := decodeAuth(r)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	token, err := h.userSvc.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			WriteJSON(w, http.StatusUnauthorized, ErrorResponse{Error: err.Error()})
			return
		}
		log.Printf("error raised in userhandlers' login: %v\n", err)
		WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	WriteJSON(w, http.StatusOK, loginResponse{Token: token})
}

func decodeAuth(r *http.Request) (*authRequest, error) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, errors.New("invalid json body")
	}
	if req.Username == "" || req.Password == "" {
		return nil, errors.New("username and password are required")
	}
	return &req, nil
}
