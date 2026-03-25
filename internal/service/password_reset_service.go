package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	db "mezzani_backend/internal/database/sqlc"
	"mezzani_backend/internal/notifications"

	"github.com/redis/go-redis/v9"
)

type PasswordResetService struct {
	Queries     *db.Queries
	Redis       *redis.Client
	Email       *notifications.EmailSender
	FrontendURL string
}

func NewPasswordResetService(
	q *db.Queries,
	r *redis.Client,
	e *notifications.EmailSender,
	frontendURL string,
) *PasswordResetService {
	return &PasswordResetService{
		Queries:     q,
		Redis:       r,
		Email:       e,
		FrontendURL: frontendURL,
	}
}

func (s *PasswordResetService) RequestReset(ctx context.Context, email string) error {
	// Check user exists — don't reveal if they don't
	_, err := s.Queries.GetStaffByEmail(ctx, email)
	if err != nil {
		return nil // silent — don't reveal if email exists
	}

	// Generate secure random token
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	token := hex.EncodeToString(b)

	// Store token in Redis with 15 min expiry
	key := fmt.Sprintf("password_reset:%s", token)
	if err := s.Redis.Set(ctx, key, email, 15*time.Minute).Err(); err != nil {
		return err
	}

	// Send email
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", s.FrontendURL, token)
	return s.Email.SendPasswordReset(email, resetLink)
}

func (s *PasswordResetService) ResetPassword(ctx context.Context, token, newPassword string) error {
	key := fmt.Sprintf("password_reset:%s", token)

	// Get email from Redis
	email, err := s.Redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return fmt.Errorf("invalid or expired reset token")
	}
	if err != nil {
		return err
	}

	// Get user
	user, err := s.Queries.GetStaffByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	// Hash new password
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}

	// Update password
	if err := s.Queries.UpdateStaffPassword(ctx, db.UpdateStaffPasswordParams{
		ID:           user.ID,
		PasswordHash: hash,
	}); err != nil {
		return err
	}

	// Delete token so it can't be reused
	s.Redis.Del(ctx, key)
	return nil
}
