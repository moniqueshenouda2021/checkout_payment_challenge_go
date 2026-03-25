package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cko-recruitment/payment-gateway-challenge-go/internal/bank"
	"github.com/cko-recruitment/payment-gateway-challenge-go/internal/models"
	"github.com/cko-recruitment/payment-gateway-challenge-go/internal/repository"
	"github.com/cko-recruitment/payment-gateway-challenge-go/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeBankClient struct {
	response bank.PaymentResponse
	err      error
}

func (f fakeBankClient) ProcessPayment(context.Context, bank.PaymentRequest) (bank.PaymentResponse, error) {
	if f.err != nil {
		return bank.PaymentResponse{}, f.err
	}

	return f.response, nil
}

func TestGetPaymentHandler(t *testing.T) {
	payment := models.PostPaymentResponse{
		Id:                 "test-id",
		PaymentStatus:      "test-successful-status",
		CardNumberLastFour: "1234",
		ExpiryMonth:        10,
		ExpiryYear:         2035,
		Currency:           "GBP",
		Amount:             100,
	}
	ps := repository.NewPaymentsRepository()
	ps.AddPayment(payment)

	payments := NewPaymentsHandler(ps)

	r := chi.NewRouter()
	r.Get("/api/payments/{id}", payments.GetHandler())

	httpServer := &http.Server{
		Addr:    ":8091",
		Handler: r,
	}

	go func() error {
		return httpServer.ListenAndServe()
	}()

	t.Run("PaymentFound", func(t *testing.T) {
		// Create a new HTTP request for testing
		req, _ := http.NewRequest("GET", "/api/payments/test-id", nil)

		// Create a new HTTP request recorder for recording the response
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		// Check the body is not nil
		assert.NotNil(t, w.Body)

		// Check the HTTP status code in the response
		if status := w.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}
		var response models.GetPaymentResponse
		require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
		assert.Equal(t, "1234", response.CardNumberLastFour)

	})
	t.Run("PaymentNotFound", func(t *testing.T) {
		// Create a new HTTP request for testing with a non-existing payment ID
		req, _ := http.NewRequest("GET", "/api/payments/NonExistingID", nil)

		// Create a new HTTP request recorder for recording the response
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		// Check the HTTP status code in the response
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
func TestPostPaymentHandler(t *testing.T) {
	originalFactory := newPaymentsService
	t.Cleanup(func() {
		newPaymentsService = originalFactory
	})

	tests := []struct {
		name           string
		client         fakeBankClient
		request        models.PostPaymentRequest
		idempotencyKey string
		expectedStatus int
		expectedResult string
	}{
		{
			name: "authorized payment",
			client: fakeBankClient{response: bank.PaymentResponse{
				Authorized:        true,
				AuthorizationCode: "auth-123",
			}},
			request: models.PostPaymentRequest{
				CardNumber:  "4242424242424241",
				ExpiryMonth: 10,
				ExpiryYear:  2035,
				Currency:    "GBP",
				Amount:      100,
				Cvv:         "123",
			},
			expectedStatus: http.StatusCreated,
			expectedResult: models.PaymentStatusAuthorized,
		},
		{
			name: "declined payment",
			client: fakeBankClient{response: bank.PaymentResponse{
				Authorized: false,
			}},
			request: models.PostPaymentRequest{
				CardNumber:  "4242424242424242",
				ExpiryMonth: 10,
				ExpiryYear:  2035,
				Currency:    "GBP",
				Amount:      100,
				Cvv:         "123",
			},
			expectedStatus: http.StatusCreated,
			expectedResult: models.PaymentStatusDeclined,
		},
		{
			name:   "rejected payment",
			client: fakeBankClient{},
			request: models.PostPaymentRequest{
				CardNumber:  "abc",
				ExpiryMonth: 10,
				ExpiryYear:  2035,
				Currency:    "GBP",
				Amount:      100,
				Cvv:         "123",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "bank unavailable",
			client: fakeBankClient{
				err: bank.ErrBankUnavailable,
			},
			request: models.PostPaymentRequest{
				CardNumber:  "4242424242424241",
				ExpiryMonth: 10,
				ExpiryYear:  2035,
				Currency:    "GBP",
				Amount:      100,
				Cvv:         "123",
			},
			expectedStatus: http.StatusBadGateway,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repository.NewPaymentsRepository()
			newPaymentsService = func(storage *repository.PaymentsRepository) *service.PaymentsService {
				return service.NewPaymentsService(storage, tt.client)
			}
			handler := NewPaymentsHandler(repo)

			router := chi.NewRouter()
			router.Post("/api/payments", handler.PostHandler())

			body, err := json.Marshal(tt.request)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/payments", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			if tt.idempotencyKey != "" {
				req.Header.Set("Idempotency-Key", tt.idempotencyKey)
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedStatus == http.StatusBadGateway || tt.expectedStatus == http.StatusBadRequest {
				var response models.ErrorResponse
				require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
				assert.NotEmpty(t, response.Error)
				return
			}

			var response models.PostPaymentResponse
			require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
			assert.Equal(t, tt.expectedResult, response.PaymentStatus)
			assert.Equal(t, strings.ToUpper(strings.TrimSpace(tt.request.Currency)), response.Currency)
			assert.Equal(t, tt.request.CardNumber[len(tt.request.CardNumber)-4:], response.CardNumberLastFour)

		})
	}
}
func TestPostPaymentHandlerIdempotency(t *testing.T) {
	originalFactory := newPaymentsService
	t.Cleanup(func() {
		newPaymentsService = originalFactory
	})

	repo := repository.NewPaymentsRepository()
	newPaymentsService = func(storage *repository.PaymentsRepository) *service.PaymentsService {
		return service.NewPaymentsService(storage, fakeBankClient{
			response: bank.PaymentResponse{Authorized: true, AuthorizationCode: "auth-1"},
		})
	}
	handler := NewPaymentsHandler(repo)

	router := chi.NewRouter()
	router.Post("/api/payments", handler.PostHandler())

	request := models.PostPaymentRequest{
		CardNumber:  "4242424242424241",
		ExpiryMonth: 10,
		ExpiryYear:  2035,
		Currency:    "GBP",
		Amount:      100,
		Cvv:         "123",
	}
	body, err := json.Marshal(request)
	require.NoError(t, err)

	firstReq := httptest.NewRequest(http.MethodPost, "/api/payments", bytes.NewReader(body))
	firstReq.Header.Set("Content-Type", "application/json")
	firstReq.Header.Set("Idempotency-Key", "same-key")
	firstResp := httptest.NewRecorder()
	router.ServeHTTP(firstResp, firstReq)

	secondReq := httptest.NewRequest(http.MethodPost, "/api/payments", bytes.NewReader(body))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.Header.Set("Idempotency-Key", "same-key")
	secondResp := httptest.NewRecorder()
	router.ServeHTTP(secondResp, secondReq)

	assert.Equal(t, http.StatusCreated, firstResp.Code)
	assert.Equal(t, http.StatusOK, secondResp.Code)

	var firstPayment models.PostPaymentResponse
	var secondPayment models.PostPaymentResponse
	require.NoError(t, json.NewDecoder(firstResp.Body).Decode(&firstPayment))
	require.NoError(t, json.NewDecoder(secondResp.Body).Decode(&secondPayment))
	assert.Equal(t, firstPayment.Id, secondPayment.Id)
}

func TestPostPaymentHandlerInvalidJSON(t *testing.T) {
	originalFactory := newPaymentsService
	t.Cleanup(func() {
		newPaymentsService = originalFactory
	})

	repo := repository.NewPaymentsRepository()
	newPaymentsService = func(storage *repository.PaymentsRepository) *service.PaymentsService {
		return service.NewPaymentsService(storage, fakeBankClient{err: errors.New("should not be called")})
	}
	handler := NewPaymentsHandler(repo)

	router := chi.NewRouter()
	router.Post("/api/payments", handler.PostHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/payments", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
func TestPostPaymentHandlerNormalizesCurrency(t *testing.T) {
	originalFactory := newPaymentsService
	t.Cleanup(func() {
		newPaymentsService = originalFactory
	})

	repo := repository.NewPaymentsRepository()
	newPaymentsService = func(storage *repository.PaymentsRepository) *service.PaymentsService {
		return service.NewPaymentsService(storage, fakeBankClient{
			response: bank.PaymentResponse{
				Authorized:        true,
				AuthorizationCode: "auth-123",
			},
		})
	}
	handler := NewPaymentsHandler(repo)

	router := chi.NewRouter()
	router.Post("/api/payments", handler.PostHandler())

	request := models.PostPaymentRequest{
		CardNumber:  "4242424242424241",
		ExpiryMonth: 10,
		ExpiryYear:  2035,
		Currency:    " gbp ",
		Amount:      100,
		Cvv:         "123",
	}

	body, err := json.Marshal(request)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/payments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response models.PostPaymentResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
	assert.Equal(t, models.PaymentStatusAuthorized, response.PaymentStatus)
	assert.Equal(t, "GBP", response.Currency)
}

func TestPostPaymentHandlerRejectsUnknownFields(t *testing.T) {
	originalFactory := newPaymentsService
	t.Cleanup(func() {
		newPaymentsService = originalFactory
	})

	repo := repository.NewPaymentsRepository()
	newPaymentsService = func(storage *repository.PaymentsRepository) *service.PaymentsService {
		return service.NewPaymentsService(storage, fakeBankClient{
			err: errors.New("bank should not be called"),
		})
	}
	handler := NewPaymentsHandler(repo)

	router := chi.NewRouter()
	router.Post("/api/payments", handler.PostHandler())

	body := bytes.NewBufferString(`{
		"card_number":"4242424242424241",
		"expiry_month":10,
		"expiry_year":2035,
		"currency":"GBP",
		"amount":100,
		"cvv":"123",
		"unexpected":"value"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/payments", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response models.ErrorResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
	assert.Equal(t, "invalid request body", response.Error)
}

func TestPostPaymentHandlerRejectsMultipleJSONObjects(t *testing.T) {
	originalFactory := newPaymentsService
	t.Cleanup(func() {
		newPaymentsService = originalFactory
	})

	repo := repository.NewPaymentsRepository()
	newPaymentsService = func(storage *repository.PaymentsRepository) *service.PaymentsService {
		return service.NewPaymentsService(storage, fakeBankClient{
			err: errors.New("bank should not be called"),
		})
	}
	handler := NewPaymentsHandler(repo)

	router := chi.NewRouter()
	router.Post("/api/payments", handler.PostHandler())

	body := bytes.NewBufferString(`{"card_number":"4242424242424241","expiry_month":10,"expiry_year":2035,"currency":"GBP","amount":100,"cvv":"123"}{"card_number":"4242424242424241","expiry_month":10,"expiry_year":2035,"currency":"GBP","amount":100,"cvv":"123"}`)

	req := httptest.NewRequest(http.MethodPost, "/api/payments", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response models.ErrorResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
	assert.Equal(t, "request body must contain a single JSON object", response.Error)
}
