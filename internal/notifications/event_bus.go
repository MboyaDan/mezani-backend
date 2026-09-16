package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/redis/go-redis/v9"
)

type EventBus struct {
	client *redis.Client
}

func NewEventBus(client *redis.Client) *EventBus {
	return &EventBus{client: client}
}

func (e *EventBus) Publish(channel string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return e.client.Publish(context.Background(), channel, data).Err()
}

func (e *EventBus) Subscribe(channels ...string) *redis.PubSub {
	return e.client.Subscribe(context.Background(), channels...)
}

type ContactMessage struct {
	Name    string
	Email   string
	Message string
}

// SendContactMessage sends a contact-form submission by email. All three
// fields are attacker-controlled (anyone can submit the public form), so
// they are HTML-escaped before being embedded in the email body — without
// this, a submitter could inject arbitrary HTML/links into an email you
// personally open and read, which is a real phishing/injection vector.
func (e *EmailSender) SendContactMessage(ctx context.Context, recipients []string, msg ContactMessage) error {
	safeName := html.EscapeString(msg.Name)
	safeEmail := html.EscapeString(msg.Email)
	safeMessage := strings.ReplaceAll(html.EscapeString(msg.Message), "\n", "<br>")

	body := resendRequest{
		From:    e.FromEmail,
		To:      recipients,
		Subject: fmt.Sprintf("New contact form message from %s", safeName),
		HTML: fmt.Sprintf(`
			<h2>New message from the Mezzani contact form</h2>
			<p><strong>Name:</strong> %s</p>
			<p><strong>Email:</strong> %s</p>
			<p><strong>Message:</strong></p>
			<p>%s</p>
		`, safeName, safeEmail, safeMessage),
	}

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.resend.com/emails", bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+e.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("resend API error: status %d", resp.StatusCode)
	}

	return nil
}
