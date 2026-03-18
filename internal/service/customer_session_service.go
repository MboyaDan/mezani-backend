package service

import (
	"context"

	db "mezzani_backend/internal/database/sqlc"

	"github.com/google/uuid"
)

type CustomerSessionService struct {
	Queries *db.Queries
}

func NewCustomerSessionService(q *db.Queries) *CustomerSessionService {
	return &CustomerSessionService{
		Queries: q,
	}
}

func (s *CustomerSessionService) JoinTableSession(
	ctx context.Context,
	tableSessionID uuid.UUID,
	name string,
) (db.CustomerSession, error) {

	customer, err := s.Queries.CreateCustomerSession(ctx, db.CreateCustomerSessionParams{
		ID:             uuid.New(),
		TableSessionID: tableSessionID,
		Name:           name,
	})

	if err != nil {
		return db.CustomerSession{}, err
	}

	return customer, nil
}

func (s *CustomerSessionService) GetCustomer(
	ctx context.Context,
	customerID uuid.UUID,
) (db.CustomerSession, error) {

	return s.Queries.GetCustomerSession(ctx, customerID)
}

func (s *CustomerSessionService) ListCustomers(
	ctx context.Context,
	tableSessionID uuid.UUID,
) ([]db.CustomerSession, error) {

	return s.Queries.ListCustomersByTableSession(ctx, tableSessionID)
}
