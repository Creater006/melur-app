package auth

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input LoginInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be a valid JSON object")
		return
	}

	clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		clientIP = ""
	}
	response, err := h.service.Login(r.Context(), input, r.UserAgent(), clientIP)
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Userdata and password are required")
	case errors.Is(err, ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid userdata or password")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to log in")
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input LogoutInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must contain a refresh_token")
		return
	}
	if err := h.service.Logout(r.Context(), input); errors.Is(err, ErrInvalidInput) {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Refresh token is required")
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to log out")
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input ForgotPasswordInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must contain userdata")
		return
	}
	response, err := h.service.ForgotPassword(r.Context(), input)
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Userdata is required")
	case errors.Is(err, ErrPasswordResetDeliveryNeeded):
		writeError(w, http.StatusServiceUnavailable, "RESET_DELIVERY_UNAVAILABLE", "Password reset delivery is not configured")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to process password reset request")
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(response)
	}
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input ResetPasswordInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must contain reset_token and new_password")
		return
	}
	err := h.service.ResetPassword(r.Context(), input)
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Reset token and a password of at least 5 characters are required")
	case errors.Is(err, ErrInvalidResetToken):
		writeError(w, http.StatusBadRequest, "INVALID_RESET_TOKEN", "Reset token is invalid or expired")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to reset password")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: errorDetail{Code: code, Message: message}})
}
