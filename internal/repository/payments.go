package repository

import (
	"sync"

	"github.com/cko-recruitment/payment-gateway-challenge-go/internal/models"
)

type PaymentsRepository struct {
	mu                  sync.RWMutex
	payments            []models.PostPaymentResponse
	idempotencyPayments map[string]string
}

func NewPaymentsRepository() *PaymentsRepository {
	return &PaymentsRepository{
		payments:            []models.PostPaymentResponse{},
		idempotencyPayments: make(map[string]string),
	}
}

func (ps *PaymentsRepository) GetPayment(id string) *models.PostPaymentResponse {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	for _, element := range ps.payments {
		if element.Id == id {
			return &element
		}
	}
	return nil
}

func (ps *PaymentsRepository) GetPaymentByIdempotencyKey(key string) *models.PostPaymentResponse {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	paymentID, ok := ps.idempotencyPayments[key]
	if !ok {
		return nil
	}

	for _, payment := range ps.payments {
		if payment.Id == paymentID {
			p := payment
			return &p
		}
	}

	return nil
}

func (ps *PaymentsRepository) AddPayment(payment models.PostPaymentResponse) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.payments = append(ps.payments, payment)
}

func (ps *PaymentsRepository) SaveIdempotencyKey(key, paymentID string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if key == "" {
		return
	}

	ps.idempotencyPayments[key] = paymentID
}
