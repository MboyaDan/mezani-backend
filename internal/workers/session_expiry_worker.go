package workers

import (
	"context"
	"time"

	db "mezzani_backend/internal/database/sqlc"
)

func StartSessionExpiryWorker(q *db.Queries) {

	ticker := time.NewTicker(1 * time.Minute)

	for range ticker.C {

		expired, err := q.GetExpiredSessions(context.Background())

		if err != nil {
			continue
		}

		for _, session := range expired {

			q.CloseTableSession(context.Background(), session.ID)
		}
	}
}
