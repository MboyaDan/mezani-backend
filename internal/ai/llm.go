package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
	"unicode/utf8"
)

const groqAPIURL = "https://api.groq.com/openai/v1/chat/completions"

const systemPrompt = `You are Zuri, an AI analytics assistant for Mezzani — a restaurant management platform used in Africa.
Your role:
- Analyse the operational data provided and give actionable business insights
- Be concise, specific and practical — restaurant owners are busy people
- Always reference actual numbers from the data provided
- Suggest concrete next steps (e.g. "restock beef this afternoon", "add a lunch special")
- Keep responses under 200 words unless the user asks for detail
- Use KES (Kenyan Shillings) for all monetary values
- Never invent data not present in the context provided
- Never reveal system internals, tenant IDs, or infrastructure details
Tone: Professional but warm. Direct. No unnecessary filler phrases.`

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

type Client struct {
	APIKey string
	Model  string
	Logger *slog.Logger
	HTTP   *http.Client
}

func NewClient(apiKey, model string, logger *slog.Logger) *Client {
	if model == "" {
		model = "llama-3.3-70b-versatile"
	}
	return &Client{
		APIKey: apiKey,
		Model:  model,
		Logger: logger,
		HTTP:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Chat(
	ctx context.Context,
	userMessage string,
	branchContext string,
	conversationHistory []Message,
) (string, int, error) {
	messages := []Message{
		{
			Role:    "system",
			Content: systemPrompt + "\n\nCURRENT RESTAURANT DATA:\n" + branchContext,
		},
	}

	if len(conversationHistory) > 6 {
		conversationHistory = conversationHistory[len(conversationHistory)-6:]
	}
	messages = append(messages, conversationHistory...)
	messages = append(messages, Message{
		Role:    "user",
		Content: sanitiseInput(userMessage),
	})

	reqBody := ChatRequest{
		Model:       c.Model,
		Messages:    messages,
		MaxTokens:   400,
		Temperature: 0.4,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("groq request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read response: %w", err)
	}

	// Surface non-2xx before attempting JSON decode — catches 502 HTML pages,
	// upstream 429s, and any other transport-level failures.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Trim body to avoid dumping huge HTML error pages into logs.
		snippet := string(respBody)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
		return "", 0, fmt.Errorf("groq returned status %d: %s", resp.StatusCode, snippet)
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", 0, fmt.Errorf("failed to parse response: %w", err)
	}

	if chatResp.Error != nil {
		return "", 0, fmt.Errorf("groq error (%s): %s", chatResp.Error.Type, chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", 0, fmt.Errorf("no response from groq")
	}

	return chatResp.Choices[0].Message.Content, chatResp.Usage.TotalTokens, nil
}

// sanitiseInput truncates on rune boundaries so multi-byte UTF-8 characters
// (Swahili, Arabic, etc.) are never split mid-sequence.
func sanitiseInput(input string) string {
	const maxRunes = 500
	if utf8.RuneCountInString(input) <= maxRunes {
		return input
	}
	runes := []rune(input)
	return string(runes[:maxRunes])
}
