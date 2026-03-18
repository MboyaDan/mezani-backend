package service

import (
	"context"

	db "mezzani_backend/internal/database/sqlc"

	"github.com/google/uuid"
)

type SharedCartService struct {
	Queries *db.Queries
}

func NewSharedCartService(q *db.Queries) *SharedCartService {
	return &SharedCartService{Queries: q}
}

func (s *SharedCartService) CreateCart(
	ctx context.Context,
	tableSessionID uuid.UUID,
	customerID uuid.UUID,
) (db.SharedCart, error) {

	cart, err := s.Queries.CreateSharedCart(ctx, db.CreateSharedCartParams{
		ID:             uuid.New(),
		TableSessionID: tableSessionID,
		CreatedBy:      customerID,
	})

	if err != nil {
		return db.SharedCart{}, err
	}

	err = s.Queries.AddCartParticipant(ctx, db.AddCartParticipantParams{
		ID:                uuid.New(),
		CartID:            cart.ID,
		CustomerSessionID: customerID,
	})

	if err != nil {
		return db.SharedCart{}, err
	}

	return cart, nil
}

func (s *SharedCartService) JoinCart(
	ctx context.Context,
	cartID uuid.UUID,
	customerID uuid.UUID,
) error {

	return s.Queries.AddCartParticipant(ctx, db.AddCartParticipantParams{
		ID:                uuid.New(),
		CartID:            cartID,
		CustomerSessionID: customerID,
	})
}

func (s *SharedCartService) AddItem(
	ctx context.Context,
	cartID uuid.UUID,
	menuItemID uuid.UUID,
	quantity int32,
	customerID uuid.UUID,
) (db.CartItem, error) {

	item, err := s.Queries.AddCartItem(ctx, db.AddCartItemParams{
		ID:         uuid.New(),
		CartID:     cartID,
		MenuItemID: menuItemID,
		Quantity:   quantity,
		AddedBy:    customerID,
	})

	if err != nil {
		return db.CartItem{}, err
	}

	return item, nil
}
