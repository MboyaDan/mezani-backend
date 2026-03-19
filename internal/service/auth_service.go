package service

import (
	"context"
	"errors"
	"time"

	db "mezzani_backend/internal/database/sqlc"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return db.StaffUser{}, err
	}

	return s.Queries.CreateStaffUser(ctx, db.CreateStaffUserParams{
		ID:           uuid.New(),
		TenantID:     tenantID,
		BranchID:     branchID,
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
		CreatedBy:    createdBy,
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

	claims := jwt.MapClaims{
		"user_id":   user.ID,
		"tenant_id": user.TenantID,
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

	tenant, err := s.Queries.CreateTenant(ctx, db.CreateTenantParams{
		ID:   uuid.New(),
		Name: restaurantName,
		Plan: "tier1",
	})
	if err != nil {
		return "", err
	}

	_, err = s.RegisterStaff(
		ctx,
		tenant.ID,
		uuid.Nil, // no branch yet
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
