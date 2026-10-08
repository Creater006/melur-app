package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = errors.New("user not found")

type User struct {
	ID           string
	Name         string
	Email        string
	Phone        string
	PasswordHash string
	IsActive     bool
	Roles        []string
}

type Repository interface {
	FindUserByUserdata(context.Context, string) (User, error)
	RecordLogin(context.Context, string, string, time.Time, string, string) error
	RevokeRefreshToken(context.Context, string) error
	CreatePasswordReset(context.Context, string, string, time.Time) error
	ResetPassword(context.Context, string, string, time.Time) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) FindUserByUserdata(ctx context.Context, userdata string) (User, error) {
	const query = `
		SELECT u.id::text, u.name, COALESCE(u.email, ''), u.phone, u.password_hash, u.is_active,
			COALESCE(array_agg(roles.name ORDER BY roles.name) FILTER (WHERE roles.name IS NOT NULL), ARRAY[]::text[])
		FROM users AS u
		LEFT JOIN user_roles AS user_roles ON user_roles.user_id = u.id
		LEFT JOIN roles AS roles ON roles.id = user_roles.role_id
		WHERE (u.phone = $1 OR lower(u.email) = lower($1)) AND u.deleted_at IS NULL
		GROUP BY u.id`

	var user User
	err := r.pool.QueryRow(ctx, query, userdata).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Phone,
		&user.PasswordHash,
		&user.IsActive,
		&user.Roles,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("find user: %w", err)
	}
	return user, nil
}

func (r *PostgresRepository) RecordLogin(ctx context.Context, userID, tokenHash string, expiresAt time.Time, userAgent, ipAddress string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin login transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx, `
		UPDATE users SET last_login_at = NOW()
		WHERE id = $1 AND is_active = TRUE AND deleted_at IS NULL`, userID)
	if err != nil {
		return fmt.Errorf("update login timestamp: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrUserNotFound
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at, user_agent, ip_address)
		VALUES ($1, $2, $3, $4, NULLIF($5, '')::inet)`,
		userID, tokenHash, expiresAt, userAgent, ipAddress)
	if err != nil {
		return fmt.Errorf("store refresh token: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit login transaction: %w", err)
	}
	return nil
}

func (r *PostgresRepository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW()
		WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func (r *PostgresRepository) CreatePasswordReset(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin password reset transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE password_reset_tokens SET used_at = NOW()
		WHERE user_id = $1 AND used_at IS NULL`, userID); err != nil {
		return fmt.Errorf("invalidate previous reset tokens: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`, userID, tokenHash, expiresAt); err != nil {
		return fmt.Errorf("insert password reset token: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password reset token: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ResetPassword(ctx context.Context, tokenHash, passwordHash string, now time.Time) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin password update transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var userID string
	err = tx.QueryRow(ctx, `
		UPDATE password_reset_tokens SET used_at = $2
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2
		RETURNING user_id::text`, tokenHash, now).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidResetToken
	}
	if err != nil {
		return fmt.Errorf("consume password reset token: %w", err)
	}

	result, err := tx.Exec(ctx, `
		UPDATE users SET password_hash = $2
		WHERE id = $1 AND is_active = TRUE AND deleted_at IS NULL`, userID, passwordHash)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrInvalidResetToken
	}
	if _, err := tx.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = $2
		WHERE user_id = $1 AND revoked_at IS NULL`, userID, now); err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE password_reset_tokens SET used_at = $2
		WHERE user_id = $1 AND used_at IS NULL`, userID, now); err != nil {
		return fmt.Errorf("invalidate other reset tokens: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password update: %w", err)
	}
	return nil
}
