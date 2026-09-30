package main

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/infrai-examples/captcha-gated-health-signup/signup"
)

type signupRequest struct {
	Email          string                    `json:"email"`
	Password       string                    `json:"password"`
	Name           string                    `json:"name"`
	WidgetRecordID string                    `json:"captcha_widget_record_id"`
	Captcha        string                    `json:"captcha_token"`
	Vendor         string                    `json:"captcha_vendor"`
	Appointment    signup.AppointmentRequest `json:"appointment"`
}

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	gate := signup.NewRegistrationGate(signup.NewInfraiClient(apiKey))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /signup", func(w http.ResponseWriter, r *http.Request) {
		var input signupRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid signup request"})
			return
		}
		idempotencyKey := r.Header.Get("Idempotency-Key")
		result, err := gate.Register(r.Context(), signup.CaptchaCheck{
			WidgetRecordID: input.WidgetRecordID,
			Token:          input.Captcha,
			Vendor:         input.Vendor,
			IP:             clientIP(r),
			Action:         "signup",
			ScoreThreshold: 0.7,
		}, signup.AccountRequest{
			Email: input.Email, Password: input.Password, Name: input.Name, IdempotencyKey: idempotencyKey,
		}, input.Appointment)
		if err != nil {
			status := http.StatusBadGateway
			var inputErr *signup.InputError
			if errors.As(err, &inputErr) {
				status = http.StatusBadRequest
			}
			var apiErr *signup.APIError
			if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
				status = apiErr.HTTPStatus
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, result)
	})

	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("health signup listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
