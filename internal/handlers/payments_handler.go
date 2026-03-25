package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/cko-recruitment/payment-gateway-challenge-go/internal/bank"
	"github.com/cko-recruitment/payment-gateway-challenge-go/internal/models"
	"github.com/cko-recruitment/payment-gateway-challenge-go/internal/repository"
	"github.com/cko-recruitment/payment-gateway-challenge-go/internal/service"
	"github.com/go-chi/chi/v5"
)

type PaymentsHandler struct {
	storage *repository.PaymentsRepository
}

var newPaymentsService = func(storage *repository.PaymentsRepository) *service.PaymentsService {
	return service.NewPaymentsService(storage, bank.NewHTTPClient("http://localhost:8080", 3*time.Second))
}

func NewPaymentsHandler(storage *repository.PaymentsRepository) *PaymentsHandler {
	return &PaymentsHandler{
		storage: storage,
	}
}

// GetHandler returns an http.HandlerFunc that handles HTTP GET requests.
// It retrieves a payment record by its ID from the storage.
// The ID is expected to be part of the URL.
func (h *PaymentsHandler) GetHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		paymentsService := newPaymentsService(h.storage)
		payment, err := paymentsService.GetPayment(id)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrPaymentNotFound):
				writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "payment not found"})
				return
			default:
				writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
				return
			}
		}

		writeJSON(w, http.StatusOK, payment)
	}
}

func (ph *PaymentsHandler) PostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var req models.PostPaymentRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
			return
		}

		if err := decoder.Decode(&struct{}{}); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "request body must contain a single JSON object"})
			return
		}

		paymentsService := newPaymentsService(ph.storage)
		payment, replayed, err := paymentsService.ProcessPayment(r.Context(), req, r.Header.Get("Idempotency-Key"))
		if err != nil {
			switch {
			case errors.Is(err, service.ErrValidation):
				writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
				return
			case errors.Is(err, bank.ErrBankUnavailable):
				writeJSON(w, http.StatusBadGateway, models.ErrorResponse{Error: "bank is currently unavailable"})
				return
			default:
				writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
				return
			}
		}

		status := http.StatusCreated
		if replayed {
			status = http.StatusOK
		}

		writeJSON(w, status, payment)
	}
}
