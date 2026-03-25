package service

import (
	"context"
	"testing"
	"time"

	"github.com/cko-recruitment/payment-gateway-challenge-go/internal/bank"
	"github.com/cko-recruitment/payment-gateway-challenge-go/internal/models"
	"github.com/cko-recruitment/payment-gateway-challenge-go/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type bankStub struct {
	response bank.PaymentResponse
	err      error
	calls    int
}

func (b *bankStub) ProcessPayment(context.Context, bank.PaymentRequest) (bank.PaymentResponse, error) {
	b.calls++
	if b.err != nil {
		return bank.PaymentResponse{}, b.err
	}

	return b.response, nil
}

func TestPaymentsServiceReturnsAuthorizedPayment(t *testing.T) {
	repo := repository.NewPaymentsRepository()
	bankClient := &bankStub{
		response: bank.PaymentResponse{
			Authorized:        true,
			AuthorizationCode: "auth-1",
		},
	}
	svc := NewPaymentsService(repo, bankClient)
	svc.now = func() time.Time {
		return time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	}

	response, replayed, err := svc.ProcessPayment(context.Background(), models.PostPaymentRequest{
		CardNumber:         "4242424242424241",
		CardNumberLastFour: 4241,
		ExpiryMonth:        10,
		ExpiryYear:         2035,
		Currency:           " gbp ",
		Amount:             1050,
		Cvv:                "123",
	}, "")

	require.NoError(t, err)
	assert.False(t, replayed)
	assert.Equal(t, models.PaymentStatusAuthorized, response.PaymentStatus)
	assert.Equal(t, 4241, response.CardNumberLastFour)
	assert.Equal(t, "GBP", response.Currency)
	assert.Equal(t, 1050, response.Amount)
	assert.NotEmpty(t, response.Id)
	assert.Equal(t, 1, bankClient.calls)
}

func TestPaymentsServiceReturnsDeclinedPayment(t *testing.T) {
	repo := repository.NewPaymentsRepository()
	bankClient := &bankStub{
		response: bank.PaymentResponse{
			Authorized: false,
		},
	}
	svc := NewPaymentsService(repo, bankClient)
	svc.now = func() time.Time {
		return time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	}

	response, replayed, err := svc.ProcessPayment(context.Background(), models.PostPaymentRequest{
		CardNumber:         "4242424242424242",
		CardNumberLastFour: 4242,
		ExpiryMonth:        10,
		ExpiryYear:         2035,
		Currency:           "GBP",
		Amount:             1050,
		Cvv:                "123",
	}, "")

	require.NoError(t, err)
	assert.False(t, replayed)
	assert.Equal(t, models.PaymentStatusDeclined, response.PaymentStatus)
	assert.Equal(t, 4242, response.CardNumberLastFour)
	assert.Equal(t, "GBP", response.Currency)
	assert.Equal(t, 1050, response.Amount)
	assert.NotEmpty(t, response.Id)
	assert.Equal(t, 1, bankClient.calls)
}

func TestPaymentsServiceRejectsInvalidRequestsWithoutCallingBank(t *testing.T) {
	repo := repository.NewPaymentsRepository()
	bankClient := &bankStub{}
	svc := NewPaymentsService(repo, bankClient)
	svc.now = func() time.Time {
		return time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	}

	response, replayed, err := svc.ProcessPayment(context.Background(), models.PostPaymentRequest{
		CardNumber:  "abc",
		ExpiryMonth: 10,
		ExpiryYear:  2035,
		Currency:    "GBP",
		Amount:      100,
		Cvv:         "123",
	}, "")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
	assert.False(t, replayed)
	assert.Equal(t, models.PostPaymentResponse{}, response)
	assert.Equal(t, 0, bankClient.calls)

}

func TestPaymentsServiceReplaysByIdempotencyKey(t *testing.T) {
	repo := repository.NewPaymentsRepository()
	bankClient := &bankStub{response: bank.PaymentResponse{Authorized: true, AuthorizationCode: "auth-1"}}
	svc := NewPaymentsService(repo, bankClient)
	svc.now = func() time.Time {
		return time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	}

	request := models.PostPaymentRequest{
		CardNumber:  "4242424242424241",
		ExpiryMonth: 10,
		ExpiryYear:  2035,
		Currency:    "GBP",
		Amount:      100,
		Cvv:         "123",
	}

	first, replayed, err := svc.ProcessPayment(context.Background(), request, "key")
	require.NoError(t, err)
	assert.False(t, replayed)

	second, replayed, err := svc.ProcessPayment(context.Background(), request, "key")
	require.NoError(t, err)
	assert.True(t, replayed)
	assert.Equal(t, first.Id, second.Id)
	assert.Equal(t, 1, bankClient.calls)
}
func TestPaymentsServiceRejectsUnsupportedCurrencyWithoutCallingBank(t *testing.T) {
	repo := repository.NewPaymentsRepository()
	bankClient := &bankStub{}
	svc := NewPaymentsService(repo, bankClient)
	svc.now = func() time.Time {
		return time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	}

	response, replayed, err := svc.ProcessPayment(context.Background(), models.PostPaymentRequest{
		CardNumber:  "4242424242424241",
		ExpiryMonth: 10,
		ExpiryYear:  2035,
		Currency:    "ABC",
		Amount:      100,
		Cvv:         "123",
	}, "")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
	assert.False(t, replayed)
	assert.Equal(t, models.PostPaymentResponse{}, response)
	assert.Equal(t, 0, bankClient.calls)
}

func TestPaymentsServiceRejectsExpiredCardWithoutCallingBank(t *testing.T) {
	repo := repository.NewPaymentsRepository()
	bankClient := &bankStub{}
	svc := NewPaymentsService(repo, bankClient)
	svc.now = func() time.Time {
		return time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	}

	response, replayed, err := svc.ProcessPayment(context.Background(), models.PostPaymentRequest{
		CardNumber:  "4242424242424241",
		ExpiryMonth: 12,
		ExpiryYear:  2025,
		Currency:    "GBP",
		Amount:      100,
		Cvv:         "123",
	}, "")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
	assert.False(t, replayed)
	assert.Equal(t, models.PostPaymentResponse{}, response)
	assert.Equal(t, 0, bankClient.calls)
}

func TestPaymentsServiceRejectsInvalidCVVWithoutCallingBank(t *testing.T) {
	repo := repository.NewPaymentsRepository()
	bankClient := &bankStub{}
	svc := NewPaymentsService(repo, bankClient)
	svc.now = func() time.Time {
		return time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	}

	response, replayed, err := svc.ProcessPayment(context.Background(), models.PostPaymentRequest{
		CardNumber:  "4242424242424241",
		ExpiryMonth: 10,
		ExpiryYear:  2035,
		Currency:    "GBP",
		Amount:      100,
		Cvv:         "12a",
	}, "")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
	assert.False(t, replayed)
	assert.Equal(t, models.PostPaymentResponse{}, response)
	assert.Equal(t, 0, bankClient.calls)
}
func TestPaymentsServiceRejectsMismatchedLastFourWithoutCallingBank(t *testing.T) {
	repo := repository.NewPaymentsRepository()
	bankClient := &bankStub{}
	svc := NewPaymentsService(repo, bankClient)
	svc.now = func() time.Time {
		return time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	}

	response, replayed, err := svc.ProcessPayment(context.Background(), models.PostPaymentRequest{
		CardNumber:         "4242424242424241",
		CardNumberLastFour: 9999,
		ExpiryMonth:        10,
		ExpiryYear:         2035,
		Currency:           "GBP",
		Amount:             100,
		Cvv:                "123",
	}, "")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
	assert.False(t, replayed)
	assert.Equal(t, models.PostPaymentResponse{}, response)
	assert.Equal(t, 0, bankClient.calls)
}
