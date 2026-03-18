package service

import (
	"context"
	"errors"
	"time"

	db "mezzani_backend/internal/database/sqlc"

	"github.com/google/uuid"
)

type TableSessionService struct {
	Queries *db.Queries
}

func NewTableSessionService(q *db.Queries) *TableSessionService {
	return &TableSessionService{
		Queries: q,
	}
}

func (s *TableSessionService) StartSession(
	ctx context.Context,
	tableID uuid.UUID,
	durationMinutes int,
) (db.TableSession, error) {

	//  Check if an active session already exists
	existing, err := s.Queries.GetActiveTableSessionByTable(ctx, tableID)

	if err == nil {
		// session already active → reuse it
		return existing, nil
	}

	// Calculate expiration time
	expiresAt := time.Now().Add(
		time.Duration(durationMinutes) * time.Minute,
	)

	// Create new session
	session, err := s.Queries.CreateTableSession(
		ctx,
		db.CreateTableSessionParams{
			ID:        uuid.New(),
			TableID:   tableID,
			ExpiresAt: expiresAt,
		},
	)

	if err != nil {
		return db.TableSession{}, err
	}

	return session, nil
}

func (s *TableSessionService) CloseSession(
	ctx context.Context,
	sessionID uuid.UUID,
) error {

	return s.Queries.CloseTableSession(ctx, sessionID)
}

func (s *TableSessionService) GetSession(
	ctx context.Context,
	id uuid.UUID,
) (db.TableSession, error) {

	session, err := s.Queries.GetTableSession(ctx, id)

	if err != nil {
		return db.TableSession{}, errors.New("session not found")
	}

	return session, nil
}

func (s *TableSessionService) Heartbeat(
	ctx context.Context,
	sessionID uuid.UUID,
) error {

	return s.Queries.ExtendTableSession(ctx, sessionID)
}
