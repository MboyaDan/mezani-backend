package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	db "mezzani_backend/internal/database/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrPlanNotFound            = errors.New("plan not found")
	ErrPaymentNotSuccessful    = errors.New("payment was not successful")
	ErrPaymentReferenceUnknown = errors.New("unknown payment reference")
	ErrAmountMismatch          = errors.New("paid amount does not match the expected plan price")
)

type SubscriptionService struct {
	Queries     *db.Queries
	Paystack    *PaystackClient
	FrontendURL string
	Logger      *slog.Logger
}

func NewSubscriptionService(q *db.Queries, paystack *PaystackClient, frontendURL string, logger *slog.Logger) *SubscriptionService {
	if logger == nil {
		logger = slog.Default()
	}
	return &SubscriptionService{
		Queries:     q,
		Paystack:    paystack,
		FrontendURL: frontendURL,
		Logger:      logger,
	}
}

func (s *SubscriptionService) ListPlans(ctx context.Context) ([]db.Plan, error) {
	plans, err := s.Queries.ListActivePlans(ctx)
	if err != nil {
		return nil, err
	}
	if plans == nil {
		plans = []db.Plan{}
	}
	return plans, nil
}

func (s *SubscriptionService) InitiateRenewal(
	ctx context.Context,
	tenantID uuid.UUID,
	planName string,
	staffID uuid.UUID,
) (string, error) {
	staff, err := s.Queries.GetStaffByID(ctx, staffID)
	if err != nil {
		return "", fmt.Errorf("failed to look up initiating staff member: %w", err)
	}

	plan, err := s.Queries.GetPlanByName(ctx, planName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrPlanNotFound
		}
		return "", err
	}

	reference := "sub_" + uuid.New().String()

	if _, err := s.Queries.CreateSubscriptionPayment(ctx, db.CreateSubscriptionPaymentParams{
		ID:                uuid.New(),
		TenantID:          tenantID,
		PlanID:            plan.ID,
		Amount:            plan.PriceKes,
		Currency:          "KES",
		PaystackReference: reference,
		InitiatedBy:       uuidToPgtype(staffID),
	}); err != nil {
		return "", err
	}

	callbackURL := s.FrontendURL + "/billing/callback"

	result, err := s.Paystack.InitializeTransaction(
		ctx,
		staff.Email,
		plan.PriceKes,
		reference,
		callbackURL,
		map[string]any{
			"tenant_id": tenantID.String(),
			"plan_name": plan.Name,
		},
	)
	if err != nil {
		s.Logger.ErrorContext(ctx, "paystack initialize failed", "error", err, "tenant_id", tenantID)
		return "", err
	}

	return result.AuthorizationURL, nil
}

// VerifyAndActivate is called by the Paystack webhook — no caller tenant
// to check against, since Paystack calling us has no tenant scope at
// all. Its authority comes entirely from the verified HMAC signature
// upstream in the handler, not from any tenant-ownership check.
func (s *SubscriptionService) VerifyAndActivate(ctx context.Context, reference string) (db.Tenant, error) {
	return s.verifyAndActivate(ctx, nil, reference)
}

// VerifyAndActivateForTenant is called by the authenticated redirect-
// fallback endpoint. callerTenantID MUST be enforced here: without this,
// any authenticated staff member could submit another tenant's payment
// reference and activate (and learn the subscription details of) an
// account that isn't theirs.
func (s *SubscriptionService) VerifyAndActivateForTenant(ctx context.Context, callerTenantID uuid.UUID, reference string) (db.Tenant, error) {
	return s.verifyAndActivate(ctx, &callerTenantID, reference)
}

func (s *SubscriptionService) verifyAndActivate(ctx context.Context, callerTenantID *uuid.UUID, reference string) (db.Tenant, error) {
	payment, err := s.Queries.GetSubscriptionPaymentByReference(ctx, reference)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Tenant{}, ErrPaymentReferenceUnknown
		}
		return db.Tenant{}, err
	}

	// Returning the SAME "unknown reference" error for both "doesn't
	// exist" and "exists but belongs to someone else" is deliberate — it
	// avoids confirming to an attacker that a given reference is even
	// valid, which a distinct "forbidden" response would leak.
	if callerTenantID != nil && payment.TenantID != *callerTenantID {
		return db.Tenant{}, ErrPaymentReferenceUnknown
	}

	if payment.Status == "success" {
		return s.Queries.GetTenantByID(ctx, payment.TenantID)
	}

	verified, err := s.Paystack.VerifyTransaction(ctx, reference)
	if err != nil {
		s.Logger.ErrorContext(ctx, "paystack verify failed", "error", err, "reference", reference)
		return db.Tenant{}, err
	}

	if !verified.Success {
		if _, err := s.Queries.MarkSubscriptionPaymentFailed(ctx, reference); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			s.Logger.ErrorContext(ctx, "failed to mark subscription payment failed", "error", err, "reference", reference)
		}
		return db.Tenant{}, ErrPaymentNotSuccessful
	}

	// Validate BOTH amount and currency — matching subunits alone would
	// still accept, say, an equal-numbered payment made in a different
	// currency than what was actually charged.
	expectedSubunits := AmountToSubunits(payment.Amount)
	if verified.AmountKobo != expectedSubunits || verified.Currency != payment.Currency {
		s.Logger.ErrorContext(ctx, "subscription payment amount/currency mismatch",
			"reference", reference,
			"expected_subunits", expectedSubunits, "paid_subunits", verified.AmountKobo,
			"expected_currency", payment.Currency, "paid_currency", verified.Currency)
		return db.Tenant{}, ErrAmountMismatch
	}

	// Marking the payment success and extending the subscription MUST
	// happen in one transaction. If they were separate statements and
	// ExtendTenantSubscription failed (e.g. the plan was deactivated
	// between initiation and verification — a real, reachable case, not
	// just an infra fault), the payment would be stuck permanently marked
	// "success" while the tenant's expiry never moved. That's
	// unrecoverable by retry: the "if payment.Status == success, return
	// early" check above would short-circuit every future webhook
	// redelivery or redirect-fallback call before extension is ever
	// retried. Wrapping both in a transaction means a failed extension
	// rolls back the success mark too, so the payment stays "pending"
	// and the next delivery attempts the whole activation again.
	tx, err := s.Queries.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.Tenant{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful Commit

	qtx := s.Queries.WithTx(tx)

	if _, err := qtx.MarkSubscriptionPaymentSuccess(ctx, reference); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.Queries.GetTenantByID(ctx, payment.TenantID)
		}
		return db.Tenant{}, err
	}

	// plan_id (not a free-floating name/period) — ExtendTenantSubscription
	// derives the actual plan name and billing period from the plans
	// table itself in the same statement, so there's no path here that
	// could set an invalid plan or a negative period.
	tenant, err := qtx.ExtendTenantSubscription(ctx, db.ExtendTenantSubscriptionParams{
		TenantID: payment.TenantID,
		PlanID:   payment.PlanID,
	})
	if err != nil {
		return db.Tenant{}, fmt.Errorf("payment succeeded but subscription extension failed: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Tenant{}, fmt.Errorf("failed to commit subscription activation: %w", err)
	}

	return tenant, nil
}
