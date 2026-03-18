package notifications

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type WhatsAppSender struct {
	Token   string
	PhoneID string
}

func NewWhatsAppSender(token, phoneID string) *WhatsAppSender {
	return &WhatsAppSender{
		Token:   token,
		PhoneID: phoneID,
	}
}

func (w *WhatsAppSender) SendMessage(to string, message string) error {

	url := "https://graph.facebook.com/v18.0/" + w.PhoneID + "/messages"

	body := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "text",
		"text": map[string]string{
			"body": message,
		},
	}

	data, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(data))

	req.Header.Set("Authorization", "Bearer "+w.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return nil
}
