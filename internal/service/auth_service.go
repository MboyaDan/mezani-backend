package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	db "mezzani_backend/internal/database/sqlc"

	"mezzani_backend/internal/auth"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type AuthService struct {
	Queries *db.Queries
	JWTKey  []byte
	Redis   *redis.Client
}

func NewAuthService(q *db.Queries, key []byte, redisClient *redis.Client) *AuthService {
	return &AuthService{
		Queries: q,
		JWTKey:  key,
		Redis:   redisClient,
	}
}

// uuidToPgtype converts a uuid.UUID to pgtype.UUID.
// Pass uuid.Nil to get a null pgtype.UUID.
func uuidToPgtype(id uuid.UUID) pgtype.UUID {
	if id == uuid.Nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

func (s *AuthService) RegisterStaff(
	ctx context.Context,
	tenantID uuid.UUID,
	branchID uuid.UUID, // pass uuid.Nil for owner
	createdBy uuid.UUID, // pass uuid.Nil for self-registered
	name string,
	email string,
	password string,
	role string,
) (db.StaffUser, error) {

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return db.StaffUser{}, err
	}

	return s.Queries.CreateStaffUser(ctx, db.CreateStaffUserParams{
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

func (s *AuthService) Login(
	ctx context.Context,
	email string,
	password string,
) (TokenPair, error) {

	user, err := s.Queries.GetStaffByEmail(ctx, email)
	if err != nil {
		return TokenPair{}, errors.New("invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	)
	if err != nil {
		return TokenPair{}, errors.New("invalid credentials")
	}

	return s.GenerateTokenPair(ctx, user)
}

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
			ID:        uuid.NewString(), // enables per-token revocation and audit
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
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
		return "", err
	}

	token := hex.EncodeToString(b)

	hash := sha256.Sum256([]byte(token))
	key := fmt.Sprintf("refresh_token:%x", hash)

	if err := s.Redis.Set(ctx, key, userID.String(), 7*24*time.Hour).Err(); err != nil {
		return "", err
	}

	return token, nil
}

func (s *AuthService) GenerateTokenPair(ctx context.Context, user db.StaffUser) (TokenPair, error) {
	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return TokenPair{}, err
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

func (s *AuthService) RefreshAccessToken(ctx context.Context, refreshToken string) (TokenPair, error) {
	hash := sha256.Sum256([]byte(refreshToken))
	key := fmt.Sprintf("refresh_token:%x", hash)

	userIDStr, err := s.Redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return TokenPair{}, errors.New("invalid or expired refresh token")
	}
	if err != nil {
		return TokenPair{}, err
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return TokenPair{}, errors.New("invalid user id in token")
	}

	user, err := s.Queries.GetStaffByID(ctx, userID)
	if err != nil {
		return TokenPair{}, errors.New("user not found")
	}

	if err := s.Redis.Del(ctx, key).Err(); err != nil {
		return TokenPair{}, err
	}

	return s.GenerateTokenPair(ctx, user)
}

func (s *AuthService) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	hash := sha256.Sum256([]byte(refreshToken))
	key := fmt.Sprintf("refresh_token:%x", hash)
	return s.Redis.Del(ctx, key).Err()
}

func (s *AuthService) RegisterOwner(
	ctx context.Context,
	restaurantName string,
	email string,
	password string,
) (TokenPair, error) {

	tenant, err := s.Queries.CreateTenant(ctx, db.CreateTenantParams{
		ID:   uuid.New(),
		Name: restaurantName,
		Plan: "tier1",
	})
	if err != nil {
		return TokenPair{}, err
	}

	_, err = s.RegisterStaff(
		ctx,
		tenant.ID,
		uuid.Nil,
		uuid.Nil,
		"Owner",
		email,
		password,
		"owner",
	)
	if err != nil {
		return TokenPair{}, err
	}

	return s.Login(ctx, email, password)
}
