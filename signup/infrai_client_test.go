package signup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyCaptchaDecodesBusinessRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization header missing")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["widget_record_id"] != "widget-123" || body["token"] != "response-token" || body["action"] != "signup" {
			t.Fatalf("body = %#v", body)
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]any{
			"ok":       false,
			"error":    map[string]string{"code": "CAPTCHA_REJECTED", "message": "verification rejected"},
			"metadata": map[string]any{},
		})
	}))
	defer server.Close()

	client := NewInfraiClient("test-key")
	client.baseURL = server.URL
	err := client.VerifyCaptcha(context.Background(), CaptchaCheck{WidgetRecordID: "widget-123", Token: "response-token", Action: "signup"})
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.HTTPStatus != http.StatusUnprocessableEntity || apiErr.Code != "CAPTCHA_REJECTED" {
		t.Fatalf("error = %#v", err)
	}
}
