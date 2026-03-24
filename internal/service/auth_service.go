package service

import (
	"context"
	"errors"
	"time"

	db "mezzani_backend/internal/database/sqlc"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	Queries *db.Queries
	JWTKey  []byte
}

func NewAuthService(q *db.Queries, key []byte) *AuthService {
	return &AuthService{
		Queries: q,
		JWTKey:  key,
	}
}

// uuidToPgtype converts a uuid.UUID to pgtype.UUID
// Pass uuid.Nil to get a null pgtype.UUID
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
		BranchID:     uuidToPgtype(branchID), // 👈 null safe
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
		CreatedBy:    uuidToPgtype(createdBy), // 👈 null safe
	})
}

func (s *AuthService) Login(
	ctx context.Context,
	email string,
	password string,
) (string, error) {

	user, err := s.Queries.GetStaffByEmail(ctx, email)
	if err != nil {
		return "", errors.New("invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	)
	if err != nil {
		return "", errors.New("invalid credentials")
	}

	// Extract branch_id safely — may be null for owners
	var branchID uuid.UUID
	if user.BranchID.Valid {
		branchID = user.BranchID.Bytes
	}

	claims := jwt.MapClaims{
		"user_id":   user.ID,
		"tenant_id": user.TenantID,
		"branch_id": branchID, // uuid.Nil for owners, real ID for staff
		"role":      user.Role,
		"exp":       time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(s.JWTKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (s *AuthService) RegisterOwner(
	ctx context.Context,
	restaurantName string,
	email string,
	password string,
) (string, error) {

	// 1. Create tenant
	tenant, err := s.Queries.CreateTenant(ctx, db.CreateTenantParams{
		ID:   uuid.New(),
		Name: restaurantName,
		Plan: "tier1",
	})
	if err != nil {
		return "", err
	}

	// 2. Create owner — no branch, created themselves
	_, err = s.RegisterStaff(
		ctx,
		tenant.ID,
		uuid.Nil, // no branch for owner
		uuid.Nil, // owner created themselves
		"Owner",
		email,
		password,
		"owner",
	)
	if err != nil {
		return "", err
	}

	return s.Login(ctx, email, password)
}
