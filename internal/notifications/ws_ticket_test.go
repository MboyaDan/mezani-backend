package notifications

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var secret = []byte("test-secret-test-secret-test-secret")

func newSvc() (*WSTicketService, *time.Time) {
	svc := NewWSTicketService(secret)
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	return svc, &now
}

func TestIssueAndRedeem(t *testing.T) {
	svc, _ := newSvc()
	tok, ttl, err := svc.Issue("user-1", "tenant-1", "branch-1")
	if err != nil {
		t.Fatal(err)
	}
	if ttl != wsTicketTTL {
		t.Fatalf("ttl = %v", ttl)
	}
	c, err := svc.Redeem(tok)
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if c.Subject != "user-1" || c.TenantID != "tenant-1" || c.BranchID != "branch-1" {
		t.Fatalf("claims = %+v", c)
	}
}

func TestTicketIsSingleUse(t *testing.T) {
	svc, _ := newSvc()
	tok, _, _ := svc.Issue("u", "t", "b")
	if _, err := svc.Redeem(tok); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Redeem(tok); err == nil {
		t.Fatal("a replayed ticket must be rejected")
	}
}

func TestTicketExpires(t *testing.T) {
	svc, now := newSvc()
	tok, _, _ := svc.Issue("u", "t", "b")
	*now = now.Add(wsTicketTTL + time.Second)
	if _, err := svc.Redeem(tok); err == nil {
		t.Fatal("an expired ticket must be rejected")
	}
}

func TestTamperedTicketRejected(t *testing.T) {
	svc, _ := newSvc()
	tok, _, _ := svc.Issue("u", "t", "b")
	parts := strings.Split(tok, ".")
	// Swap the branch in the payload while keeping the original signature.
	forged, _, _ := svc.Issue("u", "t", "OTHER-BRANCH")
	fp := strings.Split(forged, ".")
	if _, err := svc.Redeem(parts[0] + "." + fp[1] + "." + parts[2]); err == nil {
		t.Fatal("a ticket with a modified payload must be rejected")
	}
}

func TestTicketFromAnotherSecretRejected(t *testing.T) {
	a := NewWSTicketService([]byte("secret-A-secret-A-secret-A-secret-A"))
	b := NewWSTicketService([]byte("secret-B-secret-B-secret-B-secret-B"))
	tok, _, _ := a.Issue("u", "t", "b")
	if _, err := b.Redeem(tok); err == nil {
		t.Fatal("a ticket signed under a different secret must be rejected")
	}
}

// An access token (same JWT secret, access-token shape) must never open a socket.
func TestAccessTokenIsNotATicket(t *testing.T) {
	svc, _ := newSvc()
	access := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"uid": "u", "tid": "t", "bid": "b", "role": "owner",
		"exp": time.Date(2026, 10, 9, 12, 10, 0, 0, time.UTC).Unix(),
	})
	signed, _ := access.SignedString(secret)
	if _, err := svc.Redeem(signed); err == nil {
		t.Fatal("an access token must not be accepted as a ticket")
	}
}

// And the reverse: a ticket must not pass access-token verification (which uses the raw secret).
func TestTicketIsNotAnAccessToken(t *testing.T) {
	svc, _ := newSvc()
	tok, _, _ := svc.Issue("u", "t", "b")
	_, err := jwt.Parse(tok, func(*jwt.Token) (interface{}, error) { return secret, nil },
		jwt.WithTimeFunc(svc.now))
	if err == nil {
		t.Fatal("a ticket must not verify under the access-token key")
	}
}

func TestWrongAudienceRejected(t *testing.T) {
	svc, now := newSvc()
	claims := WSTicketClaims{
		TenantID: "t", BranchID: "b",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "u", ID: "x", Audience: jwt.ClaimStrings{"something-else"},
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
	}
	signed, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(svc.key)
	if _, err := svc.Redeem(signed); err == nil {
		t.Fatal("wrong audience must be rejected")
	}
}

func TestMissingExpiryRejected(t *testing.T) {
	svc, _ := newSvc()
	claims := WSTicketClaims{
		TenantID: "t", BranchID: "b",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "u", ID: "x", Audience: jwt.ClaimStrings{wsTicketAudience},
		},
	}
	signed, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(svc.key)
	if _, err := svc.Redeem(signed); err == nil {
		t.Fatal("a ticket with no expiry must be rejected")
	}
}

func TestAlgNoneRejected(t *testing.T) {
	svc, now := newSvc()
	claims := WSTicketClaims{
		TenantID: "t", BranchID: "b",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "u", ID: "x", Audience: jwt.ClaimStrings{wsTicketAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
	}
	signed, _ := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := svc.Redeem(signed); err == nil {
		t.Fatal("an unsigned (alg=none) ticket must be rejected")
	}
}

func TestGarbageRejected(t *testing.T) {
	svc, _ := newSvc()
	for _, g := range []string{"", "abc", "a.b.c", strings.Repeat("x", 5000)} {
		if _, err := svc.Redeem(g); err == nil {
			t.Fatalf("garbage %q must be rejected", g)
		}
	}
}

func TestUsedTicketsArePruned(t *testing.T) {
	svc, now := newSvc()
	tok, _, _ := svc.Issue("u", "t", "b")
	_, _ = svc.Redeem(tok)
	if len(svc.used) != 1 {
		t.Fatalf("used = %d", len(svc.used))
	}
	*now = now.Add(time.Minute)
	other, _, _ := svc.Issue("u", "t", "b")
	_, _ = svc.Redeem(other)
	if len(svc.used) != 1 {
		t.Fatalf("expired entry should have been pruned, used = %d", len(svc.used))
	}
}

func TestAuthorizeBranchSubscription(t *testing.T) {
	const tenant, other, branchA, branchB = "tenant-1", "tenant-2", "branch-A", "branch-B"
	cases := []struct {
		name                                      string
		role, tokenBranch, tokenTenant, requested string
		branchTenant                              string
		want                                      error
	}{
		{"owner, any branch of own tenant", "owner", "", tenant, branchA, tenant, nil},
		{"manager pinned to the branch", "manager", branchA, tenant, branchA, tenant, nil},
		{"kitchen pinned to the branch", "kitchen", branchA, tenant, branchA, tenant, nil},
		{"waiter pinned to the branch", "waiter", branchA, tenant, branchA, tenant, nil},
		{"cashier pinned to the branch", "cashier", branchA, tenant, branchA, tenant, nil},
		{"pinned staff asking for a sibling branch", "kitchen", branchA, tenant, branchB, tenant, ErrWSForbiddenBranch},
		{"owner asking for another tenant's branch", "owner", "", tenant, branchA, other, ErrWSForbiddenBranch},
		{"branch does not exist", "owner", "", tenant, branchA, "", ErrWSForbiddenBranch},
		{"missing tenant in token", "owner", "", "", branchA, tenant, ErrWSForbiddenBranch},
		{"superadmin has no business here", "superadmin", "", tenant, branchA, tenant, ErrWSForbiddenRole},
		{"unknown role", "intern", "", tenant, branchA, tenant, ErrWSForbiddenRole},
		{"empty role", "", "", tenant, branchA, tenant, ErrWSForbiddenRole},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AuthorizeBranchSubscription(tc.role, tc.tokenBranch, tc.tokenTenant, tc.requested, tc.branchTenant)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
