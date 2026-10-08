package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidInput                = errors.New("invalid input")
	ErrInvalidCredentials          = errors.New("invalid credentials")
	ErrInvalidResetToken           = errors.New("invalid or expired reset token")
	ErrPasswordResetDeliveryNeeded = errors.New("password reset delivery is not configured")
)

const (
	accessTokenLifetime  = 15 * time.Minute
	refreshTokenLifetime = 30 * 24 * time.Hour
	resetTokenLifetime   = 1 * time.Hour
	resetPasswordMinLen  = 5
)

type Service struct {
	repository       Repository
	jwtSecret        []byte
	developmentMode  bool
	resetTokenSender PasswordResetTokenSender
}

type ServiceOptions struct {
	DevelopmentMode  bool
	ResetTokenSender PasswordResetTokenSender
}

type PasswordResetTokenSender interface {
	SendPasswordResetToken(context.Context, User, string) error
}

type LoginInput struct {
	Userdata string `json:"userdata"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	TokenType    string      `json:"token_type"`
	ExpiresIn    int64       `json:"expires_in"`
	User         UserProfile `json:"user"`
}

type LogoutInput struct {
	RefreshToken string `json:"refresh_token"`
}

type ForgotPasswordInput struct {
	Userdata string `json:"userdata"`
}

type ForgotPasswordResponse struct {
	Message    string `json:"message"`
	ResetToken string `json:"reset_token,omitempty"`
}

type ResetPasswordInput struct {
	ResetToken  string `json:"reset_token"`
	NewPassword string `json:"new_password"`
}

type UserProfile struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Email string   `json:"email,omitempty"`
	Phone string   `json:"phone"`
	Roles []string `json:"roles"`
}

type accessClaims struct {
	Roles []string `json:"roles"`
	jwt.RegisteredClaims
}

func NewService(repository Repository, jwtSecret string) (*Service, error) {
	return NewServiceWithOptions(repository, jwtSecret, ServiceOptions{})
}

func NewServiceWithOptions(repository Repository, jwtSecret string, options ServiceOptions) (*Service, error) {
	if len(jwtSecret) < 32 {
		return nil, errors.New("JWT_SECRET must be at least 32 bytes")
	}
	return &Service{
		repository:       repository,
		jwtSecret:        []byte(jwtSecret),
		developmentMode:  options.DevelopmentMode,
		resetTokenSender: options.ResetTokenSender,
	}, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput, userAgent, ipAddress string) (LoginResponse, error) {
	input.Userdata = strings.TrimSpace(input.Userdata)
	if input.Userdata == "" || len(input.Userdata) > 255 || input.Password == "" || len(input.Password) > 1024 {
		return LoginResponse{}, ErrInvalidInput
	}

	user, err := s.repository.FindUserByUserdata(ctx, input.Userdata)
	if errors.Is(err, ErrUserNotFound) {
		verifyDummyPassword(input.Password)
		return LoginResponse{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResponse{}, fmt.Errorf("load login account: %w", err)
	}

	passwordMatches := VerifyPassword(input.Password, user.PasswordHash)
	if !passwordMatches || !user.IsActive {
		return LoginResponse{}, ErrInvalidCredentials
	}

	now := time.Now().UTC()
	accessToken, err := s.createAccessToken(user, now)
	if err != nil {
		return LoginResponse{}, fmt.Errorf("create access token: %w", err)
	}
	refreshBytes := make([]byte, 32)
	if _, err := rand.Read(refreshBytes); err != nil {
		return LoginResponse{}, fmt.Errorf("generate refresh token: %w", err)
	}
	refreshToken := base64.RawURLEncoding.EncodeToString(refreshBytes)
	refreshExpiresAt := now.Add(refreshTokenLifetime)
	tokenHash := sha256.Sum256([]byte(refreshToken))
	if err := s.repository.RecordLogin(ctx, user.ID, hex.EncodeToString(tokenHash[:]), refreshExpiresAt, userAgent, ipAddress); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return LoginResponse{}, ErrInvalidCredentials
		}
		return LoginResponse{}, fmt.Errorf("record login: %w", err)
	}

	return LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTokenLifetime.Seconds()),
		User: UserProfile{
			ID:    user.ID,
			Name:  user.Name,
			Email: user.Email,
			Phone: user.Phone,
			Roles: user.Roles,
		},
	}, nil
}

func (s *Service) Logout(ctx context.Context, input LogoutInput) error {
	if input.RefreshToken == "" || len(input.RefreshToken) > 512 {
		return ErrInvalidInput
	}
	return s.repository.RevokeRefreshToken(ctx, tokenHash(input.RefreshToken))
}

func (s *Service) ForgotPassword(ctx context.Context, input ForgotPasswordInput) (ForgotPasswordResponse, error) {
	input.Userdata = strings.TrimSpace(input.Userdata)
	if input.Userdata == "" || len(input.Userdata) > 255 {
		return ForgotPasswordResponse{}, ErrInvalidInput
	}
	if !s.developmentMode && s.resetTokenSender == nil {
		return ForgotPasswordResponse{}, ErrPasswordResetDeliveryNeeded
	}

	response := ForgotPasswordResponse{Message: "If an account matches that userdata, password reset instructions will be sent."}
	user, err := s.repository.FindUserByUserdata(ctx, input.Userdata)
	if errors.Is(err, ErrUserNotFound) || err == nil && !user.IsActive {
		return response, nil
	}
	if err != nil {
		return ForgotPasswordResponse{}, fmt.Errorf("find password reset account: %w", err)
	}

	rawToken, err := newOpaqueToken()
	if err != nil {
		return ForgotPasswordResponse{}, fmt.Errorf("generate password reset token: %w", err)
	}
	if err := s.repository.CreatePasswordReset(ctx, user.ID, tokenHash(rawToken), time.Now().UTC().Add(resetTokenLifetime)); err != nil {
		return ForgotPasswordResponse{}, fmt.Errorf("store password reset token: %w", err)
	}
	if s.resetTokenSender != nil {
		if err := s.resetTokenSender.SendPasswordResetToken(ctx, user, rawToken); err != nil {
			return ForgotPasswordResponse{}, fmt.Errorf("send password reset token: %w", err)
		}
	} else {
		response.ResetToken = rawToken
	}
	return response, nil
}

func (s *Service) ResetPassword(ctx context.Context, input ResetPasswordInput) error {
	fmt.Println(len(input.NewPassword), len(input.ResetToken), len(input.NewPassword) < resetPasswordMinLen, len(input.NewPassword), resetPasswordMinLen)
	if input.ResetToken == "" || len(input.ResetToken) > 512 || len(input.NewPassword) < resetPasswordMinLen || len(input.NewPassword) < 5 {
		return ErrInvalidInput
	}
	passwordHash, err := HashPassword(input.NewPassword)
	if err != nil {
		return fmt.Errorf("hash reset password: %w", err)
	}
	if err := s.repository.ResetPassword(ctx, tokenHash(input.ResetToken), passwordHash, time.Now().UTC()); err != nil {
		return err
	}
	return nil
}

func newOpaqueToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func (s *Service) createAccessToken(user User, now time.Time) (string, error) {
	claims := accessClaims{
		Roles: user.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "melur-api",
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenLifetime)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}
