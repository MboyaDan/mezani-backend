package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

const paystackBaseURL = "https://api.paystack.co"

type PaystackClient struct {
	SecretKey  string
	HTTPClient *http.Client
}

func NewPaystackClient(secretKey string) *PaystackClient {
	return &PaystackClient{
		SecretKey:  secretKey,
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
	}
}

type initializeTransactionRequest struct {
	Email       string         `json:"email"`
	Amount      int64          `json:"amount"`
	Currency    string         `json:"currency"`
	Reference   string         `json:"reference"`
	CallbackURL string         `json:"callback_url"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type initializeTransactionResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		AuthorizationURL string `json:"authorization_url"`
		AccessCode       string `json:"access_code"`
		Reference        string `json:"reference"`
	} `json:"data"`
}

type InitializeResult struct {
	AuthorizationURL string
	AccessCode       string
	Reference        string
}

// AmountToSubunits converts a KES amount to the subunit format Paystack
// requires. Paystack's own API reference states this applies uniformly
// across every currency they support (multiply by 100) — there is no
// currency-specific exception, including for KES.
func AmountToSubunits(amountKES float64) int64 {
	return int64(math.Round(amountKES * 100))
}

func (p *PaystackClient) InitializeTransaction(
	ctx context.Context,
	email string,
	amountKES float64,
	reference string,
	callbackURL string,
	metadata map[string]any,
) (InitializeResult, error) {
	reqBody := initializeTransactionRequest{
		Email:       email,
		Amount:      AmountToSubunits(amountKES),
		Currency:    "KES",
		Reference:   reference,
		CallbackURL: callbackURL,
		Metadata:    metadata,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return InitializeResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, paystackBaseURL+"/transaction/initialize", bytes.NewReader(body))
	if err != nil {
		return InitializeResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.SecretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return InitializeResult{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return InitializeResult{}, err
	}

	var parsed initializeTransactionResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return InitializeResult{}, fmt.Errorf("paystack: failed to parse response: %w", err)
	}

	if resp.StatusCode != http.StatusOK || !parsed.Status {
		return InitializeResult{}, fmt.Errorf("paystack: initialize failed: %s", parsed.Message)
	}

	return InitializeResult{
		AuthorizationURL: parsed.Data.AuthorizationURL,
		AccessCode:       parsed.Data.AccessCode,
		Reference:        parsed.Data.Reference,
	}, nil
}

type verifyTransactionResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Status    string `json:"status"`
		Reference string `json:"reference"`
		Amount    int64  `json:"amount"`
		Currency  string `json:"currency"`
	} `json:"data"`
}

type VerifyResult struct {
	Success    bool
	AmountKobo int64
	Currency   string
	RawStatus  string
}

// VerifyTransaction is the ONLY source of truth for whether a payment
// actually succeeded. Never trust a webhook payload's own "status" field
// or a client-supplied claim — always re-verify against this endpoint
// using the reference before extending anyone's subscription.
func (p *PaystackClient) VerifyTransaction(ctx context.Context, reference string) (VerifyResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, paystackBaseURL+"/transaction/verify/"+reference, nil)
	if err != nil {
		return VerifyResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.SecretKey)

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return VerifyResult{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return VerifyResult{}, err
	}

	var parsed verifyTransactionResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return VerifyResult{}, fmt.Errorf("paystack: failed to parse verify response: %w", err)
	}

	if resp.StatusCode != http.StatusOK || !parsed.Status {
		return VerifyResult{}, fmt.Errorf("paystack: verify failed: %s", parsed.Message)
	}

	return VerifyResult{
		Success:    parsed.Data.Status == "success",
		AmountKobo: parsed.Data.Amount,
		Currency:   parsed.Data.Currency,
		RawStatus:  parsed.Data.Status,
	}, nil
}

// VerifyWebhookSignature checks Paystack's x-paystack-signature header.
// An empty SecretKey is explicitly rejected regardless of caller/
// environment: HMAC with an empty key is a fixed, trivially computable
// function, so without this check anyone who noticed the key was blank
// could forge a valid signature for any payload of their choosing.
func (p *PaystackClient) VerifyWebhookSignature(rawBody []byte, signatureHeader string) bool {
	if p.SecretKey == "" {
		return false
	}
	mac := hmac.New(sha512.New, []byte(p.SecretKey))
	mac.Write(rawBody)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signatureHeader))
}
