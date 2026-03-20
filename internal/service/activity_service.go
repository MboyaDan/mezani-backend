package service

import (
	"context"
	"encoding/json"
	"net/netip"

	db "mezzani_backend/internal/database/sqlc"

	"github.com/google/uuid"
)

type ActivityService struct {
	Queries *db.Queries
}

func NewActivityService(q *db.Queries) *ActivityService {
	return &ActivityService{
		Queries: q,
	}
}

type ActivityParams struct {
	StaffID    uuid.UUID
	BranchID   uuid.UUID
	Action     string
	EntityType string
	EntityID   uuid.UUID
	OldData    any
	NewData    any
	IPAddress  string
	Note       string
}

func (s *ActivityService) Log(ctx context.Context, p ActivityParams) {
	params := db.CreateActivityParams{
		ID:         uuid.New(),
		StaffID:    p.StaffID,
		BranchID:   p.BranchID,
		Action:     p.Action,
		EntityType: p.EntityType,
		EntityID:   p.EntityID,
	}

	if p.OldData != nil {
		if b, err := json.Marshal(p.OldData); err == nil {
			params.OldData = b
		}
	}

	if p.NewData != nil {
		if b, err := json.Marshal(p.NewData); err == nil {
			params.NewData = b
		}
	}

	if p.IPAddress != "" {
		if addr, err := netip.ParseAddr(p.IPAddress); err == nil {
			params.IpAddress = addr
		}
	}

	if p.Note != "" {
		params.Note = p.Note
	}
	// fire and forget — activity logging should never block or fail a request
	s.Queries.CreateActivity(ctx, params)
}
