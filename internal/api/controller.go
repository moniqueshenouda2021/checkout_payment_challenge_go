package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cko-recruitment/payment-gateway-challenge-go/docs"
	"github.com/cko-recruitment/payment-gateway-challenge-go/internal/handlers"
	httpSwagger "github.com/swaggo/http-swagger"
)

type pong struct {
	Message string `json:"message"`
}

// PingHandler godoc
// @Summary Health ping
// @Description Simple ping endpoint
// @Tags health
// @Produce json
// @Success 200 {object} pong
// @Router /ping [get]
// PingHandler returns an http.HandlerFunc that handles HTTP Ping GET requests.
func (a *Api) PingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(pong{Message: "pong"}); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

// SwaggerHandler returns an http.HandlerFunc that handles HTTP Swagger related requests.
func (a *Api) SwaggerHandler() http.HandlerFunc {
	return httpSwagger.Handler(
		httpSwagger.URL(fmt.Sprintf("http://%s/swagger/doc.json", docs.SwaggerInfo.Host)),
	)
}

// GetPaymentHandler godoc
// @Summary Get payment by ID
// @Description Retrieve a previously processed payment
// @Tags payments
// @Produce json
// @Param id path string true "Payment ID"
// @Success 200 {object} models.GetPaymentResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/payments/{id} [get]
// GetPaymentHandler returns an http.HandlerFunc that handles Payments GET requests.
func (a *Api) GetPaymentHandler() http.HandlerFunc {
	h := handlers.NewPaymentsHandler(a.paymentsRepo)

	return h.GetHandler()
}

// PostPaymentHandler godoc
// @Summary Process a payment
// @Description Process a payment through the acquiring bank simulator. Invalid requests are returned as Rejected without being sent to the bank.
// @Tags payments
// @Accept json
// @Produce json
// @Param Idempotency-Key header string false "Idempotency key"
// @Param payment body models.PostPaymentRequest true "Payment request"
// @Success 200 {object} models.PostPaymentResponse "Idempotent replay"
// @Success 201 {object} models.PostPaymentResponse "Authorized, Declined, or Rejected payment"
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Failure 502 {object} models.ErrorResponse
// @Router /api/payments [post]
// PostPaymentHandler returns an http.HandlerFunc that handles Payments POST requests.
func (a *Api) PostPaymentHandler() http.HandlerFunc {
	h := handlers.NewPaymentsHandler(a.paymentsRepo)

	return h.PostHandler()
}

// LivenessHandler godoc
// @Summary Liveness probe
// @Description Check whether the service is alive
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health/live [get]
// LivenessHandler returns an http.HandlerFunc that checks the liveness.
func (a *Api) LivenessHandler() http.HandlerFunc {
	h := handlers.NewHealthHandler()
	return h.LivenessHandler()
}

// ReadinessHandler godoc
// @Summary Readiness probe
// @Description Check whether the service is ready
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health/ready [get]
// ReadinessHandler returns an http.HandlerFunc that checks the readiness.
func (a *Api) ReadinessHandler() http.HandlerFunc {
	h := handlers.NewHealthHandler()
	return h.ReadinessHandler()
}
