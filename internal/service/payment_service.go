package service

import (
	"context"
	"errors"
	"log"

	db "mezzani_backend/internal/database/sqlc"
	"mezzani_backend/internal/domain"
	"mezzani_backend/internal/notifications"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrPaymentAlreadyPending    = errors.New("a payment is already pending for this bill")
	ErrUnauthorizedTableSession = errors.New("table session does not belong to your tenant")
	ErrNothingToPay             = errors.New("no outstanding balance for this table session")
	ErrTableSessionNotFound     = errors.New("table session not found")
)

const pgUniqueViolation = "23505"

type PaymentService struct {
	Queries  *db.Queries
	Billing  *BillingService
	EventBus *notifications.EventBus
	Activity *ActivityService
}

func NewPaymentService(
	q *db.Queries,
	billing *BillingService,
	eventBus *notifications.EventBus,
	activity *ActivityService,
) *PaymentService {
	return &PaymentService{
		Queries:  q,
		Billing:  billing,
		EventBus: eventBus,
		Activity: activity,
	}
}

func (s *PaymentService) InitiateCashPayment(
	ctx context.Context,
	tableSessionID uuid.UUID,
	requestingTenantID uuid.UUID,
	initiatedBy *uuid.UUID,
) (db.Payment, error) {

	ownership, err := s.Queries.GetTableSessionOwnership(ctx, tableSessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Payment{}, ErrTableSessionNotFound
		}
		return db.Payment{}, err
	}
	if ownership.TenantID != requestingTenantID {
		return db.Payment{}, ErrUnauthorizedTableSession
	}

	// propagate — it must NOT be silently treated as "safe to proceed".
	if _, err := s.Queries.GetPendingPaymentByTableSession(ctx, tableSessionID); err == nil {
		return db.Payment{}, ErrPaymentAlreadyPending
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return db.Payment{}, err
	}

	amount, err := s.Queries.GetTableSessionOutstandingTotal(ctx, tableSessionID)
	if err != nil {
		return db.Payment{}, err
	}
	if amount <= 0 {
		return db.Payment{}, ErrNothingToPay
	}

	var initiatedByPg pgtype.UUID
	if initiatedBy != nil {
		initiatedByPg = uuidToPgtype(*initiatedBy)
	}

	payment, err := s.Queries.CreatePayment(ctx, db.CreatePaymentParams{
		ID:             uuid.New(),
		TenantID:       ownership.TenantID,
		BranchID:       ownership.BranchID,
		TableSessionID: tableSessionID,
		Method:         string(domain.PaymentMethodCash),
		Amount:         amount,
		InitiatedBy:    initiatedByPg,
	})
	if err != nil {
		// The partial unique index (one_pending_payment_per_session) is the
		// TRUE authoritative guard against duplicates — it closes the race
		// where two requests both pass the fast-path check above before
		// either has inserted. Translate that specific DB conflict into the
		// same error the fast-path returns, so callers/handlers only ever
		// need to handle ErrPaymentAlreadyPending, not a raw DB error too.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation &&
			pgErr.ConstraintName == "one_pending_payment_per_session" {
			return db.Payment{}, ErrPaymentAlreadyPending
		}
		return db.Payment{}, err
	}

	event := map[string]string{
		"type":             "payment_initiated",
		"payment_id":       payment.ID.String(),
		"table_session_id": tableSessionID.String(),
		"method":           string(domain.PaymentMethodCash),
	}
	if err := s.EventBus.Publish("payments.initiated", event); err != nil {
		log.Println("failed to publish payment.initiated event:", err)
	}

	return payment, nil
}

// ConfirmCashPayment is called by a cashier once they've physically
// received the cash. It marks the payment confirmed, then delegates to the
// existing, already-tested BillingService.CloseBill to mark orders paid
// and close the table session — no duplicated bill-closing logic.
func (s *PaymentService) ConfirmCashPayment(ctx context.Context, paymentID uuid.UUID) (db.Payment, error) {

	staffID, branchID := staffFromContext(ctx)

	payment, err := s.Queries.ConfirmPayment(ctx, db.ConfirmPaymentParams{
		ID:          paymentID,
		ConfirmedBy: uuidToPgtype(staffID),
	})
	if err != nil {
		return db.Payment{}, err
	}

	if err := s.Billing.CloseBill(ctx, payment.TableSessionID); err != nil {
		return db.Payment{}, err
	}

	s.Activity.Log(ctx, ActivityParams{
		StaffID:    staffID,
		BranchID:   branchID,
		Action:     "payment.confirm",
		EntityType: "payment",
		EntityID:   payment.ID,
	})

	event := map[string]string{
		"type":             "payment_confirmed",
		"payment_id":       payment.ID.String(),
		"table_session_id": payment.TableSessionID.String(),
	}
	if err := s.EventBus.Publish("payments.confirmed", event); err != nil {
		log.Println("failed to publish payment.confirmed event:", err)
	}

	return payment, nil
}

// GetPendingCashPayments powers the cashier's "awaiting confirmation" queue.
func (s *PaymentService) GetPendingCashPayments(ctx context.Context, branchID uuid.UUID) ([]db.Payment, error) {
	payments, err := s.Queries.GetPendingPaymentsByBranch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	// sqlc's generated :many query leaves this nil (not an empty slice)
	// when there are zero rows, which Go's encoding/json then serializes
	// as JSON `null` instead of `[]`. The frontend calls .map() on the
	// response, which throws on null. Zero pending payments is the most
	// common case (e.g. right after a cashier confirms the only one), so
	// this bites immediately if left as a raw passthrough.
	if payments == nil {
		payments = []db.Payment{}
	}
	return payments, nil
}
