package signup

import (
	"context"
	"encoding/json"
	"fmt"
)

type CaptchaCheck struct {
	WidgetRecordID string
	Token          string
	Vendor         string
	IP             string
	Action         string
	ScoreThreshold float64
}

type AccountRequest struct {
	Email          string
	Password       string
	Name           string
	IdempotencyKey string
}

type AppointmentRequest struct {
	Reference string `json:"reference"`
	StartAt   string `json:"start_at"`
}

type Registration struct {
	Account      json.RawMessage `json:"account"`
	Appointment  string          `json:"appointment_state"`
	Notification Notification    `json:"notification"`
}

type Notification struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Message   string `json:"message"`
}

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

type InputError struct {
	Message string
}

func (e *InputError) Error() string { return e.Message }

func (e *APIError) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

type Gateway interface {
	VerifyCaptcha(context.Context, CaptchaCheck) error
	CreateAccount(context.Context, AccountRequest) (json.RawMessage, error)
}

type RegistrationGate struct {
	gateway Gateway
}

func NewRegistrationGate(gateway Gateway) *RegistrationGate {
	return &RegistrationGate{gateway: gateway}
}

func (g *RegistrationGate) Register(ctx context.Context, captcha CaptchaCheck, account AccountRequest, appointment AppointmentRequest) (Registration, error) {
	if captcha.WidgetRecordID == "" || captcha.Token == "" || account.Email == "" || appointment.Reference == "" || appointment.StartAt == "" {
		return Registration{}, &InputError{Message: "captcha widget record ID, captcha token, email, appointment reference, and start time are required"}
	}
	if account.IdempotencyKey == "" {
		return Registration{}, &InputError{Message: "idempotency key is required"}
	}

	if err := g.gateway.VerifyCaptcha(ctx, captcha); err != nil {
		return Registration{}, err
	}
	created, err := g.gateway.CreateAccount(ctx, account)
	if err != nil {
		return Registration{}, err
	}

	return Registration{
		Account:     created,
		Appointment: "requested",
		Notification: Notification{
			Kind:      "appointment_request_received",
			Reference: appointment.Reference,
			Message:   "Your appointment request was received. A scheduling update will follow.",
		},
	}, nil
}
