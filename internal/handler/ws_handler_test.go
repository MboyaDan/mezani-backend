package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	db "mezzani_backend/internal/database/sqlc"
	"mezzani_backend/internal/notifications"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type fakeBranches struct {
	branch db.Branch
	err    error
}

func (f fakeBranches) GetBranchByID(context.Context, uuid.UUID) (db.Branch, error) {
	return f.branch, f.err
}

// callIssueTicket runs IssueTicket as a caller with the given identity and returns the response.
func callIssueTicket(t *testing.T, lookup BranchLookup, role, tenantID, pinnedBranch, requestedBranch string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	h := &WSHandler{
		Tickets:  notifications.NewWSTicketService([]byte("test-secret-test-secret-test-secret")),
		Branches: lookup,
	}

	r := gin.New()
	r.POST("/ws/ticket", func(c *gin.Context) {
		// Stand in for AuthMiddleware.
		c.Set("user_id", "user-1")
		c.Set("tenant_id", tenantID)
		c.Set("role", role)
		c.Set("branch_id", pinnedBranch)
	}, h.IssueTicket)

	req := httptest.NewRequest(http.MethodPost, "/ws/ticket",
		strings.NewReader(`{"branch_id":"`+requestedBranch+`"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestIssueTicket(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	branch := uuid.New()
	sibling := uuid.New()

	owned := fakeBranches{branch: db.Branch{ID: branch, TenantID: tenantA}}

	cases := []struct {
		name     string
		lookup   BranchLookup
		role     string
		tenant   string
		pinned   string
		request  string
		wantCode int
	}{
		{"owner gets a ticket for their branch", owned, "owner", tenantA.String(), "", branch.String(), http.StatusOK},
		{"pinned kitchen gets a ticket for its own branch", owned, "kitchen", tenantA.String(), branch.String(), branch.String(), http.StatusOK},
		{"pinned staff cannot ask for a sibling branch", owned, "kitchen", tenantA.String(), sibling.String(), branch.String(), http.StatusForbidden},
		{"another tenant's branch is refused", owned, "owner", tenantB.String(), "", branch.String(), http.StatusForbidden},
		{"superadmin role is refused", owned, "superadmin", tenantA.String(), "", branch.String(), http.StatusForbidden},

		// A missing branch is an authorization answer: same uniform 403.
		{"unknown branch is a 403", fakeBranches{err: pgx.ErrNoRows}, "owner", tenantA.String(), "", branch.String(), http.StatusForbidden},
		// Anything else is OUR failure and must not look like a denial.
		{"database failure is a 500, not a 403", fakeBranches{err: errors.New("connection refused")}, "owner", tenantA.String(), "", branch.String(), http.StatusInternalServerError},
		{"timeout is a 500, not a 403", fakeBranches{err: context.DeadlineExceeded}, "owner", tenantA.String(), "", branch.String(), http.StatusInternalServerError},

		{"malformed branch id is a 400", owned, "owner", tenantA.String(), "", "not-a-uuid", http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := callIssueTicket(t, tc.lookup, tc.role, tc.tenant, tc.pinned, tc.request)
			if w.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tc.wantCode, w.Body.String())
			}
			if tc.wantCode == http.StatusOK {
				if !strings.Contains(w.Body.String(), `"ticket"`) {
					t.Fatalf("missing ticket in body: %s", w.Body.String())
				}
				if got := w.Header().Get("Cache-Control"); got != "no-store" {
					t.Fatalf("Cache-Control = %q, want no-store", got)
				}
			} else if strings.Contains(w.Body.String(), `"ticket"`) {
				t.Fatalf("a refused request must not return a ticket: %s", w.Body.String())
			}
		})
	}
}
