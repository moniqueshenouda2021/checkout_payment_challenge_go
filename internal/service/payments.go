package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cko-recruitment/payment-gateway-challenge-go/internal/bank"
	"github.com/cko-recruitment/payment-gateway-challenge-go/internal/models"
	"github.com/cko-recruitment/payment-gateway-challenge-go/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrValidation      = errors.New("validation failed")
	ErrPaymentNotFound = errors.New("payment not found")
)

var allowedCurrencies = map[string]struct{}{
	"GBP": {},
	"USD": {},
	"EUR": {},
}

func normalizeCurrency(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}

type PaymentsService struct {
	repo       *repository.PaymentsRepository
	bankClient bank.Client
	now        func() time.Time
}

func NewPaymentsService(repo *repository.PaymentsRepository, bankClient bank.Client) *PaymentsService {
	return &PaymentsService{
		repo:       repo,
		bankClient: bankClient,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *PaymentsService) GetPayment(id string) (models.GetPaymentResponse, error) {
	payment := s.repo.GetPayment(id)
	if payment == nil {
		return models.GetPaymentResponse{}, ErrPaymentNotFound
	}

	return models.GetPaymentResponse(*payment), nil
}

func (s *PaymentsService) ProcessPayment(ctx context.Context, req models.PostPaymentRequest, idempotencyKey string) (models.PostPaymentResponse, bool, error) {
	if idempotencyKey != "" {
		if payment := s.repo.GetPaymentByIdempotencyKey(idempotencyKey); payment != nil {
			return *payment, true, nil
		}
	}

	if err := validatePaymentRequest(req, s.now()); err != nil {
		return models.PostPaymentResponse{}, false, fmt.Errorf("%w: %v", ErrValidation, err)
	}

	bankResp, err := s.bankClient.ProcessPayment(ctx, bank.PaymentRequest{
		CardNumber: req.CardNumber,
		ExpiryDate: fmt.Sprintf("%02d/%04d", req.ExpiryMonth, req.ExpiryYear),
		Currency:   normalizeCurrency(req.Currency),
		Amount:     int64(req.Amount),
		CVV:        req.Cvv,
	})
	if err != nil {
		return models.PostPaymentResponse{}, false, err
	}

	status := models.PaymentStatusDeclined
	if bankResp.Authorized {
		status = models.PaymentStatusAuthorized
	}

	payment := newPayment(req, status)
	s.repo.AddPayment(payment)
	s.repo.SaveIdempotencyKey(idempotencyKey, payment.Id)

	return payment, false, nil
}

func newPayment(req models.PostPaymentRequest, status string) models.PostPaymentResponse {
	cardLastFour, _ := extractCardLastFour(req.CardNumber)

	return models.PostPaymentResponse{
		Id:                 uuid.NewString(),
		PaymentStatus:      status,
		CardNumberLastFour: cardLastFour,
		ExpiryMonth:        req.ExpiryMonth,
		ExpiryYear:         req.ExpiryYear,
		Currency:           normalizeCurrency(req.Currency),
		Amount:             req.Amount,
	}
}

func validatePaymentRequest(req models.PostPaymentRequest, now time.Time) error {
	if !isDigitsOnly(req.CardNumber) || len(req.CardNumber) < 14 || len(req.CardNumber) > 19 {

		return errors.New("card_number must be 14-19 digits")
	}

	if req.Amount <= 0 {
		return errors.New("amount must be greater than zero")
	}

	currency := normalizeCurrency(req.Currency)
	if _, ok := allowedCurrencies[currency]; !ok {
		return errors.New("currency must be one of GBP, USD, EUR")
	}

	if req.ExpiryMonth < 1 || req.ExpiryMonth > 12 {
		return errors.New("expiry_month must be between 1 and 12")
	}

	if req.ExpiryYear < now.Year() || (req.ExpiryYear == now.Year() && req.ExpiryMonth < int(now.Month())) {
		return errors.New("card has expired")
	}

	if !isDigitsOnly(req.Cvv) || len(req.Cvv) < 3 || len(req.Cvv) > 4 {
		return errors.New("cvv must be 3 or 4 digits")
	}

	return nil
}

func isDigitsOnly(value string) bool {
	if value == "" {
		return false
	}

	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}
func extractCardLastFour(cardNumber string) (string, error) {
	if len(cardNumber) < 4 {
		return "", errors.New("card_number must be at least 4 digits")
	}

	return cardNumber[len(cardNumber)-4:], nil
}
