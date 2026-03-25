package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type AlertService struct {
	BotToken string
	ChatID   string
}

func NewAlertService(botToken, chatID string) *AlertService {
	return &AlertService{BotToken: botToken, ChatID: chatID}
}

func (a *AlertService) Send(level, title, message string) error {
	emoji := map[string]string{
		"critical": "🔴",
		"warning":  "🟡",
		"info":     "🟢",
	}[level]

	text := fmt.Sprintf("%s *%s*\n\n%s\n\n`%s`",
		emoji,
		title,
		message,
		time.Now().Format("2006-01-02 15:04:05 UTC"),
	)

	payload := map[string]any{
		"chat_id":    a.ChatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", a.BotToken)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("telegram API error: status %d", resp.StatusCode)
	}

	return nil
}

func (a *AlertService) Critical(title, message string) {
	a.Send("critical", title, message)
}

func (a *AlertService) Warning(title, message string) {
	a.Send("warning", title, message)
}

func (a *AlertService) Info(title, message string) {
	a.Send("info", title, message)
}
