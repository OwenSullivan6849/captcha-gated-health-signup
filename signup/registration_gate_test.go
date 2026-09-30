package signup

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeGateway struct {
	verifyErr  error
	createErr  error
	verifyCall int
	createCall int
}

func (f *fakeGateway) VerifyCaptcha(context.Context, CaptchaCheck) error {
	f.verifyCall++
	return f.verifyErr
}

func (f *fakeGateway) CreateAccount(context.Context, AccountRequest) (json.RawMessage, error) {
	f.createCall++
	return json.RawMessage(`{"id":"user_123"}`), f.createErr
}

func TestRegistrationDecision(t *testing.T) {
	check := CaptchaCheck{WidgetRecordID: "widget-123", Token: "captcha-response", Vendor: "turnstile", Action: "signup", ScoreThreshold: 0.7}
	account := AccountRequest{Email: "patient@example.test", Password: "long-passphrase", Name: "Taylor", IdempotencyKey: "signup-01"}
	appointment := AppointmentRequest{Reference: "appt-01", StartAt: "2026-10-03T09:30:00Z"}

	tests := []struct {
		name        string
		gateway     *fakeGateway
		wantState   string
		wantCreates int
		wantErr     bool
	}{
		{name: "verified signup requests appointment", gateway: &fakeGateway{}, wantState: "requested", wantCreates: 1},
		{name: "captcha rejection stops account creation", gateway: &fakeGateway{verifyErr: errors.New("captcha rejected")}, wantCreates: 0, wantErr: true},
		{name: "account rejection does not request appointment", gateway: &fakeGateway{createErr: errors.New("account rejected")}, wantCreates: 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewRegistrationGate(tt.gateway).Register(context.Background(), check, account, appointment)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got.Appointment != tt.wantState {
				t.Fatalf("appointment state = %q, want %q", got.Appointment, tt.wantState)
			}
			if tt.gateway.createCall != tt.wantCreates {
				t.Fatalf("account creates = %d, want %d", tt.gateway.createCall, tt.wantCreates)
			}
			if !tt.wantErr && got.Notification.Reference != appointment.Reference {
				t.Fatalf("notification reference = %q", got.Notification.Reference)
			}
		})
	}
}
