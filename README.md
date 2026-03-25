# Payment Gateway Challenge - Implementation Notes

## Executive Summary

This solution implements a payment gateway API in Go that processes payments, retrieves previously processed payments, validates input before calling the downstream bank simulator, and documents the public API with Swagger.

The implementation is intentionally structured to be easy to understand, test, and extend. Business logic lives in the service layer, HTTP behavior lives in the handlers, persistence is isolated behind a repository, and the external bank integration is isolated behind a client abstraction.

## Index

1. Scope and Functionality
2. Architecture
3. Implementation Details
4. Assumptions
5. Hosting and Operations
6. Security Considerations
7. Maintainability and Extension
8. Testing
9. Future Improvements

## 1. Scope and Functionality

The implemented API supports:

- `POST /api/payments` to process a payment
- `GET /api/payments/{id}` to retrieve a previously processed payment
- `GET /ping` for a simple reachability check
- `GET /health/live` for liveness
- `GET /health/ready` for readiness
- Swagger UI at `GET /swagger/index.html`

Payment outcomes supported:

- `Authorized`
- `Declined`
- Validation failures returned as `400 Bad Request`
- Downstream bank unavailability returned as `502 Bad Gateway`

Key behaviors implemented:

- Request body validation before calling the bank simulator
- Currency normalization and validation against a restricted set
- Idempotency support via the `Idempotency-Key` header
- Retrieval of stored payments by ID
- Structured request logging
- Generated Swagger documentation aligned with the implementation

## 2. Architecture

The solution is organized into clear layers:

### API Layer

Files:
- `internal/api/api.go`
- `internal/api/controller.go`

Responsibilities:
- router setup
- middleware registration
- route registration
- Swagger exposure

### Handler Layer

Files:
- `internal/handlers/payments_handler.go`
- `internal/handlers/health_handler.go`

Responsibilities:
- decode request bodies
- validate request shape at the HTTP boundary
- read path parameters and headers
- map service outcomes to HTTP status codes and JSON responses

### Service Layer

Files:
- `internal/service/payments.go`

Responsibilities:
- business validation
- currency normalization
- idempotency handling
- payment outcome selection
- retrieval logic
- orchestration of repository and bank client usage

### Repository Layer

Files:
- `internal/repository/payments.go`

Responsibilities:
- in-memory storage of processed payments
- in-memory storage of idempotency-key mappings
- concurrency-safe access using a mutex

### Bank Client Layer

Files:
- `internal/bank/client.go`

Responsibilities:
- outbound HTTP communication with the bank simulator
- request marshaling
- response decoding
- timeout handling
- downstream failure mapping

### Observability Layer

Files:
- `internal/observability/middleware.go`

Responsibilities:
- structured request logging
- request duration logging
- request ID logging
- response status logging

## 3. Implementation Details

### Request Validation

The service validates:

- card number is numeric and between 14 and 19 digits
- `card_number_last_four` matches the supplied card number when provided
- amount is greater than zero
- currency is one of `GBP`, `USD`, `EUR`
- expiry month is between 1 and 12
- card is not expired
- CVV is numeric and 3 or 4 digits

### Currency Handling

Currency is normalized by trimming whitespace and converting to uppercase before validation, storage, and forwarding to the bank simulator.

### Idempotency

When an `Idempotency-Key` is supplied:

- the repository is checked first for a previously processed payment
- if a match exists, the original payment is returned
- the first request returns `201 Created`
- subsequent replay requests return `200 OK`

### Retrieval

`GET /api/payments/{id}` retrieves a payment from in-memory storage through the service layer. If the payment does not exist, the API returns `404 Not Found` with a JSON error body.

### Error Behavior

- malformed JSON returns `400 Bad Request`
- multiple JSON objects in the request body return `400 Bad Request`
- business validation failures return `400 Bad Request`
- bank unavailability returns `502 Bad Gateway`
- unexpected internal failures return `500 Internal Server Error`

## 4. Assumptions

The following assumptions were made during implementation:

- The client sends `amount` already expressed in the minor currency unit.
  - Example: `1` means `0.01`
  - Example: `1050` means `10.50`
- `amount` is expected to be an integer and is not converted from a decimal major-unit format inside the gateway.
- The supported currencies for this challenge are limited to `GBP`, `USD`, and `EUR`.
- The challenge permits in-memory persistence, so durable database storage was intentionally not introduced.
- The bank simulator is treated as the downstream acquiring-bank dependency, and the gateway is responsible for validating requests before sending valid requests to it.
- `card_number_last_four` remains part of the current public request model by design choice, but it is validated against the supplied card number when present.
- The health endpoints are intentionally lightweight at this stage.
  - Ping is a simple reachability check.
  - Liveness confirms the process is alive.
  - Readiness is the place where dependency checks could be added if the service moved closer to production.

## 5. Hosting and Operations

Current local hosting model:

- API runs on `http://localhost:8090`
- bank simulator runs on `http://localhost:8080`
- Swagger UI is served by the API

Operational support currently included:

- request ID middleware
- panic recovery middleware
- request timeout middleware
- structured request logging
- liveness and readiness endpoints

## 6. Security Considerations

Current security-related behavior:

- full card number is accepted for processing but not returned in payment responses
- only the last four digits are returned in stored and retrieved payment responses
- CVV is not persisted in payment responses
- request body size is limited
- unknown JSON fields are rejected
- structured error responses are returned

Out of scope for this challenge implementation:

- authentication and authorization
- TLS termination
- production-grade secret management
- PCI-grade storage and operational controls

## 7. Maintainability and Extension

The codebase is designed to be maintainable and open for extension.

### Maintainability

- HTTP concerns are separated from business logic
- business logic is centralized in the service layer
- persistence is isolated behind a repository abstraction
- downstream communication is isolated behind a bank client abstraction
- structured logging and health endpoints improve operational clarity
- tests exist at both the handler and service layers

### Extension

The implementation is open for extension in several ways:

- in-memory storage can be replaced with persistent storage behind the repository
- additional currencies or business rules can be added in the service layer
- downstream providers can be added behind the bank client interface
- health/readiness checks can be expanded without changing the payment flow
- observability can be expanded with metrics and tracing

## 8. Testing

### Automated Tests

The codebase includes handler tests and service tests.

Handler tests cover:

- payment retrieval
- authorized payment flow
- declined payment flow
- invalid request handling
- mismatched last-four handling
- bank unavailable behavior
- idempotency behavior
- malformed JSON
- unknown fields
- multiple JSON objects in one request body

Service tests cover:

- authorized payment behavior
- declined payment behavior
- invalid request rejection before bank call
- unsupported currency rejection
- expired card rejection
- invalid CVV rejection
- mismatched last-four rejection
- idempotent replay behavior

### Final Test Result

Final verification command:

```powershell
go test ./...
```

Result: all tests passed successfully in the active repository.

## 9. Future Improvements

The following improvements were identified during implementation and review:

- Return an array of validation errors instead of only the first validation error encountered, to improve client feedback for invalid requests.
- Remove `card_number_last_four` from the public request model and derive it entirely from `card_number` in the service layer.
- If `card_number_last_four` remains part of the contract, continue validating that it matches the final four digits of the supplied `card_number`.
- Expand readiness checks to validate important dependencies if the service moves toward a production-like deployment.
- Introduce durable persistence instead of in-memory storage.
- Introduce a dedicated internal domain model rather than storing API-shaped response models in the repository.
- Add authentication and authorization if access control becomes required.
- Add HTTPS and stronger production-grade operational controls.
- Add retry, backoff, or circuit-breaker behavior around the downstream bank client.
- Add richer observability such as metrics and traces.

## Closing Summary

The final implementation is functionally complete for the challenge scope, documented, tested, and structured to be maintainable and extensible. The code passes the automated test suite, uses a clear layered architecture, and includes explicit assumptions and future improvement notes for reviewers.
