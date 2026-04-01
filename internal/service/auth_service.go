package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"mezzani_backend/internal/auth"
	"mezzani_backend/internal/common"
	db "mezzani_backend/internal/database/sqlc"
)

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type AuthService struct {
	Queries         *db.Queries
	JWTKey          []byte
	Redis           *redis.Client
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Logger          *slog.Logger
}

func NewAuthService(q *db.Queries, key []byte, redisClient *redis.Client, logger *slog.Logger) *AuthService {
	if logger == nil {
		logger = slog.Default()
	}

	return &AuthService{
		Queries:         q,
		JWTKey:          key,
		Redis:           redisClient,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Logger:          logger,
	}
}

// ---------------- HELPERS ----------------

func uuidToPgtype(id uuid.UUID) pgtype.UUID {
	if id == uuid.Nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

// isValidEmail uses net/mail for proper RFC 5322 validation.
// Replaces the previous strings.Contains check.
func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func isValidPassword(password string) bool {
	return len(password) >= 8
}

// ---------------- REGISTER ----------------

// registerStaffWithTx is the internal implementation used by both RegisterStaff
// and RegisterOwner (via transaction). It accepts a *db.Queries that may or may
// not be bound to a transaction, keeping all business logic in one place.
func (s *AuthService) registerStaffWithTx(
	ctx context.Context,
	q *db.Queries,
	tenantID uuid.UUID,
	branchID uuid.UUID,
	createdBy uuid.UUID,
	name string,
	email string,
	password string,
	role string,
) (db.StaffUser, error) {

	email = strings.ToLower(strings.TrimSpace(email))

	if !isValidEmail(email) {
		return db.StaffUser{}, common.ErrInvalidInput
	}

	if !isValidPassword(password) {
		return db.StaffUser{}, common.ErrInvalidInput
	}

	// [#1] Verify tenant exists before creating staff so callers get a clean
	// ErrNotFound instead of a raw Postgres foreign-key violation.
	_, err := q.GetTenantByID(ctx, tenantID)
	if err != nil {
		s.Logger.ErrorContext(ctx, "tenant not found during staff registration", "tenant_id", tenantID)
		return db.StaffUser{}, common.ErrNotFound
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		s.Logger.ErrorContext(ctx, "failed to hash password", "error", err)
		return db.StaffUser{}, common.ErrInternalServer
	}

	return q.CreateStaffUser(ctx, db.CreateStaffUserParams{
		ID:           uuid.New(),
		TenantID:     tenantID,
		BranchID:     uuidToPgtype(branchID),
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
		CreatedBy:    uuidToPgtype(createdBy),
	})
}

// RegisterStaff creates a staff user under an existing tenant. The tenant must
// already exist or ErrNotFound is returned.
func (s *AuthService) RegisterStaff(
	ctx context.Context,
	tenantID uuid.UUID,
	branchID uuid.UUID,
	createdBy uuid.UUID,
	name string,
	email string,
	password string,
	role string,
) (db.StaffUser, error) {
	return s.registerStaffWithTx(ctx, s.Queries, tenantID, branchID, createdBy, name, email, password, role)
}

// RegisterOwner creates a new tenant and its first owner staff account inside a
// single transaction. If staff creation fails the tenant is rolled back, so no
// orphaned tenants can exist.
// The new owner is automatically logged in and issued a token pair.
func (s *AuthService) RegisterOwner(
	ctx context.Context,
	restaurantName string,
	email string,
	password string,
) (TokenPair, error) {

	email = strings.ToLower(strings.TrimSpace(email))

	if !isValidEmail(email) {
		return TokenPair{}, common.ErrInvalidInput
	}

	if !isValidPassword(password) {
		return TokenPair{}, common.ErrInvalidInput
	}

	// Begin transaction so tenant + owner are created atomically.
	tx, err := s.Queries.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		s.Logger.ErrorContext(ctx, "failed to begin transaction", "error", err)
		return TokenPair{}, common.ErrInternalServer
	}
	// Rollback is a no-op after a successful Commit, so this is always safe.
	defer tx.Rollback(ctx)

	qtx := s.Queries.WithTx(tx)

	tenant, err := qtx.CreateTenant(ctx, db.CreateTenantParams{
		ID:   uuid.New(),
		Name: restaurantName,
		Plan: "tier1",
	})
	if err != nil {
		s.Logger.ErrorContext(ctx, "failed to create tenant", "error", err)
		return TokenPair{}, common.ErrInternalServer
	}

	_, err = s.registerStaffWithTx(
		ctx,
		qtx,
		tenant.ID,
		uuid.Nil,
		uuid.Nil,
		"Owner",
		email,
		password,
		"owner",
	)
	if err != nil {
		// registerStaffWithTx already logged the detail. Rollback fires via defer.
		return TokenPair{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		s.Logger.ErrorContext(ctx, "failed to commit owner registration transaction", "error", err)
		return TokenPair{}, common.ErrInternalServer
	}

	return s.Login(ctx, email, password)
}

// ---------------- LOGIN ----------------

func (s *AuthService) Login(
	ctx context.Context,
	email string,
	password string,
) (TokenPair, error) {

	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.Queries.GetStaffByEmail(ctx, email)
	if err != nil {
		s.Logger.ErrorContext(ctx, "user lookup failed", "email", email)
		return TokenPair{}, common.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	); err != nil {
		s.Logger.InfoContext(ctx, "invalid password", "user_id", user.ID)
		return TokenPair{}, common.ErrInvalidCredentials
	}

	return s.GenerateTokenPair(ctx, user)
}

// ---------------- TOKENS ----------------

func (s *AuthService) generateAccessToken(user db.StaffUser) (string, error) {
	var branchID string
	if user.BranchID.Valid {
		branchID = uuid.UUID(user.BranchID.Bytes).String()
	}

	claims := auth.Claims{
		UserID:   user.ID.String(),
		TenantID: user.TenantID.String(),
		BranchID: branchID,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			ID:        uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "mezzani-api",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.JWTKey)
}

func (s *AuthService) generateRefreshToken(ctx context.Context, userID uuid.UUID) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		s.Logger.ErrorContext(ctx, "failed to generate refresh token", "error", err)
		return "", common.ErrInternalServer
	}

	token := hex.EncodeToString(b)

	hash := sha256.Sum256([]byte(token))
	key := fmt.Sprintf("refresh_token:%x", hash)

	if err := s.Redis.Set(ctx, key, userID.String(), s.RefreshTokenTTL).Err(); err != nil {
		s.Logger.ErrorContext(ctx, "failed to store refresh token", "error", err)
		return "", common.ErrInternalServer
	}

	return token, nil
}

func (s *AuthService) GenerateTokenPair(ctx context.Context, user db.StaffUser) (TokenPair, error) {
	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		s.Logger.ErrorContext(ctx, "failed to generate access token", "user_id", user.ID)
		return TokenPair{}, common.ErrInternalServer
	}

	refreshToken, err := s.generateRefreshToken(ctx, user.ID)
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// ---------------- REFRESH ----------------

// RefreshAccessToken validates the incoming refresh token, deletes it (rotation),
// and issues a fresh token pair. Token rotation is already in place — if a stolen
// token is used after the legitimate holder has refreshed, the Redis key will be
// gone and the attacker gets ErrInvalidToken.
func (s *AuthService) RefreshAccessToken(ctx context.Context, refreshToken string) (TokenPair, error) {
	hash := sha256.Sum256([]byte(refreshToken))
	key := fmt.Sprintf("refresh_token:%x", hash)

	userIDStr, err := s.Redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return TokenPair{}, common.ErrInvalidToken
	}
	if err != nil {
		s.Logger.ErrorContext(ctx, "redis error during refresh", "error", err)
		return TokenPair{}, common.ErrInternalServer
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return TokenPair{}, common.ErrInvalidToken
	}

	user, err := s.Queries.GetStaffByID(ctx, userID)
	if err != nil {
		return TokenPair{}, common.ErrNotFound
	}

	if err := s.Redis.Del(ctx, key).Err(); err != nil {
		s.Logger.ErrorContext(ctx, "failed to delete refresh token", "user_id", userID)
		return TokenPair{}, common.ErrInternalServer
	}

	return s.GenerateTokenPair(ctx, user)
}

// ---------------- LOGOUT ----------------

func (s *AuthService) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	hash := sha256.Sum256([]byte(refreshToken))
	key := fmt.Sprintf("refresh_token:%x", hash)

	if err := s.Redis.Del(ctx, key).Err(); err != nil {
		s.Logger.ErrorContext(ctx, "failed to revoke refresh token", "error", err)
		return common.ErrInternalServer
	}

	return nil
}

// TODO(future): RevokeAllSessions(ctx, userID) — requires storing a Redis Set of
// token keys per user (e.g. "user_tokens:{userID}") so all can be deleted in one
// pass. Defer until session management requirements are clearer.
