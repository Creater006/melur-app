package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"
)

func TestLogoutRevokesRefreshTokenHash(t *testing.T) {
	repository := &fakeRepository{}
	service, err := NewService(repository, "test-secret-at-least-32-bytes-long")
	if err != nil {
		t.Fatalf("create auth service: %v", err)
	}

	const refreshToken = "raw-refresh-token"
	if err := service.Logout(context.Background(), LogoutInput{RefreshToken: refreshToken}); err != nil {
		t.Fatalf("logout: %v", err)
	}
	expectedHash := sha256.Sum256([]byte(refreshToken))
	if repository.revokedTokenHash != hex.EncodeToString(expectedHash[:]) {
		t.Fatal("logout did not revoke the hashed refresh token")
	}
}

func TestForgotPasswordReturnsDevelopmentTokenAndStoresHash(t *testing.T) {
	repository := &fakeRepository{user: User{ID: "user-123", IsActive: true}}
	service, err := NewServiceWithOptions(repository, "test-secret-at-least-32-bytes-long", ServiceOptions{DevelopmentMode: true})
	if err != nil {
		t.Fatalf("create auth service: %v", err)
	}

	response, err := service.ForgotPassword(context.Background(), ForgotPasswordInput{Userdata: "9000000000"})
	if err != nil {
		t.Fatalf("request password reset: %v", err)
	}
	if response.ResetToken == "" {
		t.Fatal("development response did not include a reset token")
	}
	if repository.resetTokenHash == response.ResetToken {
		t.Fatal("raw reset token was stored instead of its hash")
	}
	if repository.resetTokenExpires.Before(time.Now()) {
		t.Fatal("reset token is already expired")
	}
}

func TestForgotPasswordRequiresDeliveryOutsideDevelopment(t *testing.T) {
	service, err := NewService(&fakeRepository{}, "test-secret-at-least-32-bytes-long")
	if err != nil {
		t.Fatalf("create auth service: %v", err)
	}
	_, err = service.ForgotPassword(context.Background(), ForgotPasswordInput{Userdata: "9000000000"})
	if !errors.Is(err, ErrPasswordResetDeliveryNeeded) {
		t.Fatalf("forgot password error = %v, want delivery unavailable", err)
	}
}

func TestResetPasswordStoresHashAndConsumesTokenHash(t *testing.T) {
	repository := &fakeRepository{}
	service, err := NewService(repository, "test-secret-at-least-32-bytes-long")
	if err != nil {
		t.Fatalf("create auth service: %v", err)
	}

	const resetToken = "raw-reset-token"
	if err := service.ResetPassword(context.Background(), ResetPasswordInput{
		ResetToken:  resetToken,
		NewPassword: "a-new-secure-password",
	}); err != nil {
		t.Fatalf("reset password: %v", err)
	}
	if !VerifyPassword("a-new-secure-password", repository.updatedPasswordHash) {
		t.Fatal("repository did not receive a valid Argon2id password hash")
	}
	expectedTokenHash := sha256.Sum256([]byte(resetToken))
	if repository.consumedResetTokenHash != hex.EncodeToString(expectedTokenHash[:]) {
		t.Fatal("reset token was not hashed before repository lookup")
	}
}
