package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type EmailSender struct {
	APIKey    string
	FromEmail string
}

func NewEmailSender(apiKey, fromEmail string) *EmailSender {
	return &EmailSender{APIKey: apiKey, FromEmail: fromEmail}
}

type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

func (e *EmailSender) SendPasswordReset(toEmail, resetLink string) error {
	body := resendRequest{
		From:    e.FromEmail,
		To:      []string{toEmail},
		Subject: "Reset your Mezzani password",
		HTML: fmt.Sprintf(`
			<h2>Password Reset</h2>
			<p>Click the link below to reset your password. This link expires in 15 minutes.</p>
			<a href="%s" style="
				background:#000;
				color:#fff;
				padding:12px 24px;
				text-decoration:none;
				border-radius:6px;
				display:inline-block;
			">Reset Password</a>
			<p>If you didn't request this, ignore this email.</p>
		`, resetLink),
	}

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewBuffer(data))
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
