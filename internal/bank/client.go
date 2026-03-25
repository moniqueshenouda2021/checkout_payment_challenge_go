package bank

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var ErrBankUnavailable = errors.New("bank unavailable")

type Client interface {
	ProcessPayment(ctx context.Context, req PaymentRequest) (PaymentResponse, error)
}

type PaymentRequest struct {
	CardNumber string `json:"card_number"`
	ExpiryDate string `json:"expiry_date"`
	Currency   string `json:"currency"`
	Amount     int64  `json:"amount"`
	CVV        string `json:"cvv"`
}

type PaymentResponse struct {
	Authorized        bool   `json:"authorized"`
	AuthorizationCode string `json:"authorization_code"`
}

type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *HTTPClient) ProcessPayment(ctx context.Context, req PaymentRequest) (PaymentResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return PaymentResponse{}, fmt.Errorf("marshal bank request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/payments", bytes.NewReader(payload))
	if err != nil {
		return PaymentResponse{}, fmt.Errorf("create bank request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return PaymentResponse{}, fmt.Errorf("%w: %v", ErrBankUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return PaymentResponse{}, fmt.Errorf("%w: unexpected status %d", ErrBankUnavailable, resp.StatusCode)
	}

	var bankResp PaymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&bankResp); err != nil {
		return PaymentResponse{}, fmt.Errorf("decode bank response: %w", err)
	}

	return bankResp, nil
}
