package auth

import "github.com/golang-jwt/jwt/v5"

type Claims struct {
	UserID          string `json:"uid"`
	TenantID        string `json:"tid"`
	TenantName      string `json:"tname"`
	BranchID        string `json:"bid,omitempty"`
	Role            string `json:"role"`
	TenantCreatedAt int64  `json:"tca"`
	Plan            string `json:"plan"`
	jwt.RegisteredClaims
}
