package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"mezzani_backend/internal/auth"
	"mezzani_backend/internal/common"
	db "mezzani_backend/internal/database/sqlc"
)

type SuperAdminService struct {
	Queries         *db.Queries
	JWTKey          []byte
	Redis           *redis.Client
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Logger          *slog.Logger
}

func NewSuperAdminService(q *db.Queries, key []byte, redisClient *redis.Client, logger *slog.Logger) *SuperAdminService {
	if logger == nil {
		logger = slog.Default()
	}
	return &SuperAdminService{
		Queries:         q,
		JWTKey:          key,
		Redis:           redisClient,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Logger:          logger,
	}
}

// ---------------- LOGIN ----------------

func (s *SuperAdminService) Login(ctx context.Context, email, password string) (TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	admin, err := s.Queries.GetPlatformAdminByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.Logger.InfoContext(ctx, "platform admin login failed: no such account")
			return TokenPair{}, common.ErrInvalidCredentials
		}
		s.Logger.ErrorContext(ctx, "platform admin lookup failed", "error", err)
		return TokenPair{}, common.ErrInternalServer
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(admin.PasswordHash),
		[]byte(password),
	); err != nil {
		s.Logger.InfoContext(ctx, "invalid password", "admin_id", admin.ID)
		return TokenPair{}, common.ErrInvalidCredentials
	}

	return s.generateTokenPair(ctx, admin)
}

// ---------------- TOKENS ----------------

func (s *SuperAdminService) generateAccessToken(admin db.PlatformAdmin) (string, error) {
	claims := auth.PlatformAdminClaims{
		AdminID: admin.ID.String(),
		Email:   admin.Email,
		Role:    "superadmin",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   admin.ID.String(),
			ID:        uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "mezzani-api",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.JWTKey)
}

func (s *SuperAdminService) generateRefreshToken(ctx context.Context, adminID uuid.UUID) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		s.Logger.ErrorContext(ctx, "failed to generate admin refresh token", "error", err)
		return "", common.ErrInternalServer
	}

	token := hex.EncodeToString(b)

	hash := sha256.Sum256([]byte(token))
	key := fmt.Sprintf("admin_refresh_token:%x", hash)

	if err := s.Redis.Set(ctx, key, adminID.String(), s.RefreshTokenTTL).Err(); err != nil {
		s.Logger.ErrorContext(ctx, "failed to store admin refresh token", "error", err)
		return "", common.ErrInternalServer
	}

	return token, nil
}

func (s *SuperAdminService) generateTokenPair(ctx context.Context, admin db.PlatformAdmin) (TokenPair, error) {
	accessToken, err := s.generateAccessToken(admin)
	if err != nil {
		s.Logger.ErrorContext(ctx, "failed to generate admin access token", "admin_id", admin.ID)
		return TokenPair{}, common.ErrInternalServer
	}

	refreshToken, err := s.generateRefreshToken(ctx, admin.ID)
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// ---------------- REFRESH ----------------

func (s *SuperAdminService) RefreshAccessToken(ctx context.Context, refreshToken string) (TokenPair, error) {
	hash := sha256.Sum256([]byte(refreshToken))
	key := fmt.Sprintf("admin_refresh_token:%x", hash)

	adminIDStr, err := s.Redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return TokenPair{}, common.ErrInvalidToken
	}
	if err != nil {
		s.Logger.ErrorContext(ctx, "redis error during admin refresh", "error", err)
		return TokenPair{}, common.ErrInternalServer
	}

	adminID, err := uuid.Parse(adminIDStr)
	if err != nil {
		return TokenPair{}, common.ErrInvalidToken
	}

	deleted, err := s.Redis.Del(ctx, key).Result()
	if err != nil {
		s.Logger.ErrorContext(ctx, "failed to delete admin refresh token", "admin_id", adminID)
		return TokenPair{}, common.ErrInternalServer
	}
	if deleted != 1 {
		return TokenPair{}, common.ErrInvalidToken
	}

	admin, err := s.Queries.GetPlatformAdminByID(ctx, adminID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TokenPair{}, common.ErrNotFound
		}
		s.Logger.ErrorContext(ctx, "platform admin lookup failed during refresh", "error", err)
		return TokenPair{}, common.ErrInternalServer
	}

	return s.generateTokenPair(ctx, admin)
}

// ---------------- ANALYTICS ----------------

func (s *SuperAdminService) GetPlatformOverview(ctx context.Context) (db.GetPlatformOverviewRow, error) {
	return s.Queries.GetPlatformOverview(ctx)
}

func (s *SuperAdminService) ListTenantsWithStats(ctx context.Context) ([]db.ListTenantsWithStatsRow, error) {
	rows, err := s.Queries.ListTenantsWithStats(ctx)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []db.ListTenantsWithStatsRow{}
	}
	return rows, nil
}
