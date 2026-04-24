package service

import (
	"context"
	"database/sql"
	"errors"
	"log"
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

	log.Println("START SESSION SERVICE CALLED")
	log.Println("TABLE ID:", tableID)

	// 1. Check if an active session already exists
	existing, err := s.Queries.GetActiveTableSessionByTable(ctx, tableID)

	if err == nil {
		log.Println("EXISTING ACTIVE SESSION FOUND:", existing.ID)
		return existing, nil
	}

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Println("ERROR CHECKING EXISTING SESSION:", err)
		return db.TableSession{}, err
	}

	log.Println("NO ACTIVE SESSION FOUND — CREATING NEW ONE")

	// 2. Calculate expiration time
	expiresAt := time.Now().Add(
		time.Duration(durationMinutes) * time.Minute,
	)

	log.Println("EXPIRES AT:", expiresAt)

	// 3. Create new session
	sessionID := uuid.New()

	log.Println("INSERTING SESSION WITH ID:", sessionID)

	session, err := s.Queries.CreateTableSession(
		ctx,
		db.CreateTableSessionParams{
			ID:        sessionID,
			TableID:   tableID,
			ExpiresAt: expiresAt,
		},
	)

	if err != nil {
		log.Println("CREATE SESSION ERROR:", err)
		return db.TableSession{}, err
	}

	log.Println("SESSION INSERTED:", session.ID)

	//  CRITICAL DEBUG CHECK
	check, err := s.Queries.GetActiveTableSessionByTable(ctx, tableID)
	log.Println("POST-INSERT CHECK RESULT:", check)
	log.Println("POST-INSERT CHECK ERROR:", err)

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

func (s *TableSessionService) GetActiveSessionByTable(ctx context.Context, tableID uuid.UUID) (db.TableSession, error) {
	return s.Queries.GetActiveTableSessionByTable(ctx, tableID)
}
