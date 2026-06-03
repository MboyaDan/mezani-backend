package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"mezzani_backend/internal/ai"
	db "mezzani_backend/internal/database/sqlc"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	aiRateLimitKey     = "ai:ratelimit:%s"    // per tenant
	aiResponseCacheKey = "ai:response:%s"     // per request hash
	aiConversationKey  = "ai:conversation:%s" // per user session

	aiRateLimit       = 20 // requests per hour per tenant
	aiResponseTTL     = 5 * time.Minute
	aiConversationTTL = 2 * time.Hour
	aiContextTTL      = 2 * time.Minute
	aiContextKey      = "ai:context:%s" // per branch
)

type AIService struct {
	Queries *db.Queries
	Redis   *redis.Client
	Client  *ai.Client
	Logger  *slog.Logger
}

func NewAIService(
	q *db.Queries,
	r *redis.Client,
	client *ai.Client,
	logger *slog.Logger,
) *AIService {
	return &AIService{
		Queries: q,
		Redis:   r,
		Client:  client,
		Logger:  logger,
	}
}

type AIChatRequest struct {
	TenantID       uuid.UUID
	BranchID       uuid.UUID
	UserID         uuid.UUID
	RestaurantName string
	Message        string
	SessionID      string
}

type AIChatResponse struct {
	Response    string         `json:"response"`
	TokensUsed  int            `json:"tokens_used"`
	FromCache   bool           `json:"from_cache"`
	ContextUsed map[string]any `json:"context_summary"`
}

func (s *AIService) Chat(ctx context.Context, req AIChatRequest) (*AIChatResponse, error) {

	// ── 1. Rate limit: atomic INCR-then-compare ───────────────────────────────
	// Increment first so concurrent requests are counted before any completes.
	// This closes the check-then-act race and also counts in-flight requests.
	limited, err := s.incrementAndCheckRateLimit(ctx, req.TenantID)
	if err != nil {
		s.Logger.WarnContext(ctx, "rate limit check failed — allowing request", "error", err)
	}
	if limited {
		return nil, fmt.Errorf("AI rate limit exceeded — %d requests per hour allowed", aiRateLimit)
	}

	// ── 2. Build branch context (cached 2 minutes) ────────────────────────────
	branchCtx, contextSummary, err := s.getOrBuildContext(ctx, req)
	if err != nil {
		s.Logger.ErrorContext(ctx, "failed to build AI context",
			"tenant_id", req.TenantID,
			"branch_id", req.BranchID,
			"error", err,
		)
		branchCtx = fmt.Sprintf("RESTAURANT: %s\nNo operational data available yet.", req.RestaurantName)
	}

	// ── 3. Load conversation history ──────────────────────────────────────────
	history := s.loadConversationHistory(ctx, req.SessionID)

	// ── 4. Response cache — skip when session has history ─────────────────────
	// Caching multi-turn replies is unsafe: the same message means something
	// different depending on what came before ("and the second one?" is context-
	// dependent). Only cache stateless (first-turn / no-session) requests.
	var cacheKey string
	if req.SessionID == "" && len(history) == 0 {
		cacheKey = s.responseHashKey(req.TenantID, req.BranchID, req.Message)
		if cached, err := s.Redis.Get(ctx, cacheKey).Result(); err == nil {
			s.Logger.InfoContext(ctx, "ai cache hit",
				"tenant_id", req.TenantID,
				"user_id", req.UserID,
			)
			return &AIChatResponse{
				Response:    cached,
				TokensUsed:  0,
				FromCache:   true,
				ContextUsed: contextSummary,
			}, nil
		}
	}

	// ── 5. Call Groq ──────────────────────────────────────────────────────────
	response, tokensUsed, err := s.Client.Chat(ctx, req.Message, branchCtx, history)
	if err != nil {
		s.Logger.ErrorContext(ctx, "groq request failed",
			"tenant_id", req.TenantID,
			"user_id", req.UserID,
			"error", err,
		)
		return nil, fmt.Errorf("AI service temporarily unavailable")
	}

	// ── 6. Update conversation history ───────────────────────────────────────
	s.appendToConversation(ctx, req.SessionID, req.Message, response)

	// ── 7. Cache stateless responses only ────────────────────────────────────
	if cacheKey != "" {
		s.Redis.Set(ctx, cacheKey, response, aiResponseTTL)
	}

	// ── 8. Log request ────────────────────────────────────────────────────────
	s.Logger.InfoContext(ctx, "ai request completed",
		"tenant_id", req.TenantID,
		"branch_id", req.BranchID,
		"user_id", req.UserID,
		"tokens_used", tokensUsed,
		"message_len", len(req.Message),
	)

	return &AIChatResponse{
		Response:    response,
		TokensUsed:  tokensUsed,
		FromCache:   false,
		ContextUsed: contextSummary,
	}, nil
}

// ── Rate limiting ─────────────────────────────────────────────────────────────

// incrementAndCheckRateLimit atomically increments the counter and returns
// true if the tenant is over the limit. The expiry is set only on the first
// request in a window so the hour starts from the first call, not the last.
func (s *AIService) incrementAndCheckRateLimit(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	key := fmt.Sprintf(aiRateLimitKey, tenantID.String())

	pipe := s.Redis.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}

	count := incrCmd.Val()

	// Set TTL only when the key is brand new (count == 1).
	// Re-setting on every call would push the window forward on each request.
	if count == 1 {
		if err := s.Redis.Expire(ctx, key, time.Hour).Err(); err != nil {
			s.Logger.WarnContext(ctx, "failed to set rate limit TTL", "error", err)
		}
	}

	return count > int64(aiRateLimit), nil
}

// ── Context building ──────────────────────────────────────────────────────────

func (s *AIService) getOrBuildContext(
	ctx context.Context,
	req AIChatRequest,
) (string, map[string]any, error) {

	cacheKey := fmt.Sprintf(aiContextKey, req.BranchID.String())

	if cached, err := s.Redis.Get(ctx, cacheKey).Result(); err == nil {
		return cached, map[string]any{"source": "cache"}, nil
	}

	branchCtx, err := ai.BuildBranchContext(
		ctx,
		s.Queries,
		req.TenantID,
		req.BranchID,
		req.RestaurantName,
		s.Logger,
	)
	if err != nil {
		return "", nil, err
	}

	formatted := branchCtx.FormatAsPromptContext()
	s.Redis.Set(ctx, cacheKey, formatted, aiContextTTL)

	summary := map[string]any{
		"total_orders":    branchCtx.TotalOrders,
		"total_revenue":   branchCtx.TotalRevenue,
		"top_item":        "",
		"low_stock_count": len(branchCtx.LowStockItems),
		"source":          "fresh",
	}
	if len(branchCtx.TopItems) > 0 {
		summary["top_item"] = branchCtx.TopItems[0].Name
	}

	return formatted, summary, nil
}

// ── Conversation history ──────────────────────────────────────────────────────

type conversationHistory struct {
	Messages []ai.Message `json:"messages"`
}

func (s *AIService) loadConversationHistory(ctx context.Context, sessionID string) []ai.Message {
	if sessionID == "" {
		return nil
	}
	key := fmt.Sprintf(aiConversationKey, sessionID)
	raw, err := s.Redis.Get(ctx, key).Result()
	if err != nil {
		return nil
	}
	var h conversationHistory
	if err := json.Unmarshal([]byte(raw), &h); err != nil {
		return nil
	}
	return h.Messages
}

func (s *AIService) appendToConversation(
	ctx context.Context,
	sessionID string,
	userMessage string,
	assistantResponse string,
) {
	if sessionID == "" {
		return
	}
	key := fmt.Sprintf(aiConversationKey, sessionID)

	history := s.loadConversationHistory(ctx, sessionID)
	history = append(history,
		ai.Message{Role: "user", Content: userMessage},
		ai.Message{Role: "assistant", Content: assistantResponse},
	)

	if len(history) > 10 {
		history = history[len(history)-10:]
	}

	raw, _ := json.Marshal(conversationHistory{Messages: history})
	s.Redis.Set(ctx, key, string(raw), aiConversationTTL)
}

// ── Cache helpers ─────────────────────────────────────────────────────────────

func (s *AIService) responseHashKey(tenantID, branchID uuid.UUID, message string) string {
	h := sha256.Sum256([]byte(tenantID.String() + branchID.String() + message))
	return fmt.Sprintf(aiResponseCacheKey, fmt.Sprintf("%x", h))
}
