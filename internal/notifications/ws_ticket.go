package notifications

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"sync"
	"time"

	"mezzani_backend/internal/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// WebSocket authentication
//
// A browser's WebSocket API cannot send an Authorization header, so the access
// token cannot be presented on the upgrade request. Putting the access token in
// the URL would be worse (it is long-lived and ends up in logs, history and
// proxies). Instead the client first calls an authenticated REST endpoint and is
// given a *ticket*: a token that is
//
//   - short-lived (wsTicketTTL),
//   - single-use (a replayed ticket is rejected),
//   - bound to one user, one tenant and one branch,
//   - signed with a key derived from the JWT secret but different from it, so a
//     ticket can never be accepted as an access token, nor an access token as a
//     ticket.
//
// The ticket is then sent once on the WebSocket URL, where it is worthless a few
// seconds later even if it leaks.

const (
	wsTicketTTL      = 30 * time.Second
	wsTicketAudience = "mezzani:ws"
	wsTicketKeyLabel = "mezzani/ws-ticket/v1"
)

var (
	ErrInvalidWSTicket   = errors.New("invalid or expired websocket ticket")
	ErrWSForbiddenRole   = errors.New("role is not allowed to subscribe to branch events")
	ErrWSForbiddenBranch = errors.New("not allowed to subscribe to this branch")
)

// WSTicketClaims is deliberately a different shape from auth.Claims.
// Subject = user id, ID (jti) = unique ticket id.
type WSTicketClaims struct {
	TenantID string `json:"tid"`
	BranchID string `json:"bid"`
	jwt.RegisteredClaims
}

type WSTicketService struct {
	key []byte
	now func() time.Time // injectable for tests

	mu   sync.Mutex
	used map[string]time.Time // jti -> ticket expiry (kept only until it would expire anyway)
}

func NewWSTicketService(jwtSecret []byte) *WSTicketService {
	mac := hmac.New(sha256.New, jwtSecret)
	mac.Write([]byte(wsTicketKeyLabel))
	return &WSTicketService{
		key:  mac.Sum(nil),
		now:  time.Now,
		used: make(map[string]time.Time),
	}
}

// Issue returns a signed ticket and how long it stays valid.
func (s *WSTicketService) Issue(userID, tenantID, branchID string) (string, time.Duration, error) {
	now := s.now()
	claims := WSTicketClaims{
		TenantID: tenantID,
		BranchID: branchID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Audience:  jwt.ClaimStrings{wsTicketAudience},
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(wsTicketTTL)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.key)
	if err != nil {
		return "", 0, err
	}
	return signed, wsTicketTTL, nil
}

// Redeem validates a ticket and consumes it. A ticket can be redeemed once.
func (s *WSTicketService) Redeem(token string) (*WSTicketClaims, error) {
	claims := &WSTicketClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims,
		func(t *jwt.Token) (interface{}, error) { return s.key, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithAudience(wsTicketAudience),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil || !parsed.Valid ||
		claims.ID == "" || claims.TenantID == "" || claims.BranchID == "" || claims.Subject == "" ||
		claims.ExpiresAt == nil {
		return nil, ErrInvalidWSTicket
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	for id, exp := range s.used { // opportunistic cleanup keeps the map tiny
		if now.After(exp) {
			delete(s.used, id)
		}
	}
	if _, replayed := s.used[claims.ID]; replayed {
		return nil, ErrInvalidWSTicket
	}
	s.used[claims.ID] = claims.ExpiresAt.Time
	return claims, nil
}

// AuthorizeBranchSubscription decides whether a signed-in staff member may listen
// to a branch's live events. It is a pure function so the rules are easy to test.
//
//	role              the caller's role from their access token
//	tokenBranchID     the branch the caller is pinned to ("" for owners)
//	tokenTenantID     the caller's tenant
//	requestedBranchID the branch they are asking for
//	branchTenantID    the tenant that actually owns the requested branch
func AuthorizeBranchSubscription(role, tokenBranchID, tokenTenantID, requestedBranchID, branchTenantID string) error {
	switch domain.Role(role) {
	case domain.RoleOwner, domain.RoleManager, domain.RoleWaiter, domain.RoleKitchen, domain.RoleCashier:
	default:
		return ErrWSForbiddenRole
	}
	// Cross-tenant: the branch must belong to the caller's own tenant.
	if branchTenantID == "" || tokenTenantID == "" || branchTenantID != tokenTenantID {
		return ErrWSForbiddenBranch
	}
	// Branch-pinned staff may only listen to their own branch. Owners have no pin.
	if tokenBranchID != "" && tokenBranchID != requestedBranchID {
		return ErrWSForbiddenBranch
	}
	return nil
}
