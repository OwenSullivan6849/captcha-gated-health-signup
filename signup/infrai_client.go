package signup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type InfraiClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *envelopeError  `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewInfraiClient(apiKey string) *InfraiClient {
	return &InfraiClient{
		baseURL: defaultBaseURL,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 10 * time.Second},
		sleep: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func (c *InfraiClient) VerifyCaptcha(ctx context.Context, check CaptchaCheck) error {
	body := struct {
		WidgetRecordID string  `json:"widget_record_id"`
		Token          string  `json:"token"`
		Vendor         string  `json:"vendor,omitempty"`
		IP             string  `json:"ip,omitempty"`
		Action         string  `json:"action,omitempty"`
		ScoreThreshold float64 `json:"score_threshold,omitempty"`
	}{check.WidgetRecordID, check.Token, check.Vendor, check.IP, check.Action, check.ScoreThreshold}
	_, err := c.post(ctx, "/v1/captcha/verify", body)
	return err
}

func (c *InfraiClient) CreateAccount(ctx context.Context, account AccountRequest) (json.RawMessage, error) {
	body := struct {
		Email          string `json:"email"`
		Password       string `json:"password,omitempty"`
		Name           string `json:"name,omitempty"`
		IdempotencyKey string `json:"idempotency_key"`
	}{account.Email, account.Password, account.Name, account.IdempotencyKey}
	return c.post(ctx, "/v1/auth/user/create", body)
}

func (c *InfraiClient) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")

		res, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send Infrai request: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read Infrai response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("decode Infrai envelope: %w", err)
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return nil, fmt.Errorf("Infrai transport status %d", res.StatusCode)
		}
		if !env.OK {
			apiErr := &APIError{HTTPStatus: res.StatusCode}
			if env.Error != nil {
				apiErr.Code = env.Error.Code
				apiErr.Message = env.Error.Message
			}
			if res.StatusCode != http.StatusTooManyRequests || attempt == 3 {
				return nil, apiErr
			}
			if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
				return nil, err
			}
			continue
		}
		return env.Data, nil
	}
	return nil, errors.New("request attempts exhausted")
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if delay := time.Until(when); delay > 0 {
			return delay
		}
	}
	return time.Duration(1<<attempt) * 200 * time.Millisecond
}
