package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type fakeRepository struct {
	user                   User
	findErr                error
	tokenHash              string
	revokedTokenHash       string
	resetTokenHash         string
	resetTokenExpires      time.Time
	updatedPasswordHash    string
	consumedResetTokenHash string
	resetErr               error
}

func (r *fakeRepository) FindUserByUserdata(context.Context, string) (User, error) {
	return r.user, r.findErr
}

func (r *fakeRepository) RecordLogin(_ context.Context, _, tokenHash string, _ time.Time, _, _ string) error {
	r.tokenHash = tokenHash
	return nil
}

func (r *fakeRepository) RevokeRefreshToken(_ context.Context, tokenHash string) error {
	r.revokedTokenHash = tokenHash
	return nil
}

func (r *fakeRepository) CreatePasswordReset(_ context.Context, _, tokenHash string, expiresAt time.Time) error {
	r.resetTokenHash = tokenHash
	r.resetTokenExpires = expiresAt
	return nil
}

func (r *fakeRepository) ResetPassword(_ context.Context, tokenHash, passwordHash string, _ time.Time) error {
	r.consumedResetTokenHash = tokenHash
	r.updatedPasswordHash = passwordHash
	return r.resetErr
}

func TestLoginCreatesAccessAndRefreshTokens(t *testing.T) {
	const secret = "test-secret-at-least-32-bytes-long"
	passwordHash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	repository := &fakeRepository{user: User{
		ID:           "user-123",
		Name:         "Sample User",
		Email:        "sample@melur.local",
		Phone:        "9000000000",
		PasswordHash: passwordHash,
		IsActive:     true,
		Roles:        []string{"user"},
	}}
	service, err := NewService(repository, secret)
	if err != nil {
		t.Fatalf("create auth service: %v", err)
	}

	response, err := service.Login(context.Background(), LoginInput{
		Userdata: "9000000000",
		Password: "correct-password",
	}, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(response.AccessToken, claims, func(token *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("parse access token: token valid=%v, error=%v", parsed != nil && parsed.Valid, err)
	}
	if claims["sub"] != "user-123" {
		t.Fatalf("token subject = %v, want user-123", claims["sub"])
	}
	if response.RefreshToken == "" || repository.tokenHash == "" || response.RefreshToken == repository.tokenHash {
		t.Fatal("expected raw refresh token in response and its hash in repository")
	}
	if response.ExpiresIn != int64((15 * time.Minute).Seconds()) {
		t.Fatalf("expires_in = %d", response.ExpiresIn)
	}
}

func TestLoginRejectsUnknownUser(t *testing.T) {
	service, err := NewService(&fakeRepository{findErr: ErrUserNotFound}, "test-secret-at-least-32-bytes-long")
	if err != nil {
		t.Fatalf("create auth service: %v", err)
	}
	_, err = service.Login(context.Background(), LoginInput{Userdata: "unknown", Password: "password"}, "", "")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("login error = %v, want invalid credentials", err)
	}
}

func TestLoginHandlerUsesUserdataField(t *testing.T) {
	passwordHash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	service, err := NewService(&fakeRepository{user: User{
		ID:           "user-123",
		Name:         "Sample User",
		Phone:        "9000000000",
		PasswordHash: passwordHash,
		IsActive:     true,
	}}, "test-secret-at-least-32-bytes-long")
	if err != nil {
		t.Fatalf("create auth service: %v", err)
	}
	handler := NewHandler(service)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"userdata":"9000000000","password":"correct-password"}`))
	response := httptest.NewRecorder()
	handler.Login(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("userdata login status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	var body LoginResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if body.AccessToken == "" {
		t.Fatal("login response has no access token")
	}

	legacyRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"identifier":"9000000000","password":"correct-password"}`))
	legacyResponse := httptest.NewRecorder()
	handler.Login(legacyResponse, legacyRequest)
	if legacyResponse.Code != http.StatusBadRequest {
		t.Fatalf("identifier login status = %d, want %d", legacyResponse.Code, http.StatusBadRequest)
	}
}
