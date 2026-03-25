package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/cko-recruitment/payment-gateway-challenge-go/internal/observability"
	"github.com/cko-recruitment/payment-gateway-challenge-go/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/sync/errgroup"
)

type Api struct {
	router       *chi.Mux
	paymentsRepo *repository.PaymentsRepository
	logger       *slog.Logger
}

func New() *Api {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	a := &Api{
		logger: logger,
	}
	a.paymentsRepo = repository.NewPaymentsRepository()
	a.setupRouter()

	return a
}

func (a *Api) Run(ctx context.Context, addr string) error {
	httpServer := &http.Server{
		Addr:        addr,
		Handler:     a.router,
		BaseContext: func(_ net.Listener) context.Context { return ctx },
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		<-ctx.Done()
		fmt.Printf("shutting down HTTP server\n")
		return httpServer.Shutdown(ctx)
	})

	g.Go(func() error {
		fmt.Printf("starting HTTP server on %s\n", addr)
		err := httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			return err
		}

		return nil
	})

	return g.Wait()
}

func (a *Api) setupRouter() {
	a.router = chi.NewRouter()
	a.router.Use(middleware.RequestID)
	a.router.Use(middleware.RealIP)
	a.router.Use(middleware.Recoverer)
	a.router.Use(middleware.Timeout(5 * time.Second))
	a.router.Use(observability.RequestLogger(a.logger))

	a.router.Get("/ping", a.PingHandler())
	a.router.Get("/health/live", a.LivenessHandler())
	a.router.Get("/health/ready", a.ReadinessHandler())
	a.router.Get("/swagger/*", a.SwaggerHandler())

	a.router.Post("/api/payments", a.PostPaymentHandler())
	a.router.Get("/api/payments/{id}", a.GetPaymentHandler())
}
