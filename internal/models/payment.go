package models

const (
	PaymentStatusAuthorized = "Authorized"
	PaymentStatusDeclined   = "Declined"
	PaymentStatusRejected   = "Rejected"
)

type PostPaymentRequest struct {
	// Primary account number. Must be 14-19 digits.
	CardNumber string `json:"card_number" binding:"required"`

	// Expiry month in the range 1-12.
	ExpiryMonth int `json:"expiry_month" binding:"required"`

	// Expiry year. Must not be in the past.
	ExpiryYear int `json:"expiry_year" binding:"required"`

	// Three-letter ISO currency code. Supported values: GBP, USD, EUR.
	Currency string `json:"currency" binding:"required"`

	// Amount in the minor currency unit, e.g. 1050 = 10.50.
	Amount int `json:"amount" binding:"required"`

	// Card verification value. Must be 3 or 4 digits.
	Cvv string `json:"cvv" binding:"required"`
}

type PostPaymentResponse struct {
	// Unique payment identifier.
	Id string `json:"id"`

	// Payment status: Authorized, Declined, or Rejected.
	PaymentStatus string `json:"payment_status"`

	// Last four digits of the card number.
	CardNumberLastFour string `json:"card_number_last_four"`

	// Expiry month in the range 1-12.
	ExpiryMonth int `json:"expiry_month"`

	// Expiry year.
	ExpiryYear int `json:"expiry_year"`

	// Three-letter ISO currency code.
	Currency string `json:"currency"`

	// Amount in the minor currency unit.
	Amount int `json:"amount"`
}

type GetPaymentResponse struct {
	// Unique payment identifier.
	Id string `json:"id"`

	// Payment status: Authorized, Declined, or Rejected.
	PaymentStatus string `json:"payment_status"`

	// Last four digits of the card number.
	CardNumberLastFour string `json:"card_number_last_four"`

	// Expiry month in the range 1-12.
	ExpiryMonth int `json:"expiry_month"`

	// Expiry year.
	ExpiryYear int `json:"expiry_year"`

	// Three-letter ISO currency code.
	Currency string `json:"currency"`

	// Amount in the minor currency unit.
	Amount int `json:"amount"`
}

type ErrorResponse struct {
	// Error message.
	Error string `json:"error"`
}
