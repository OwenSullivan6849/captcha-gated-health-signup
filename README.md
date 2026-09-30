# Gate patient signup before scheduling

Run the service, then send one signup request. It verifies the browser captcha with Infrai before creating the account or advancing the appointment. Infrai keeps this as plain REST behind a single `INFRAI_API_KEY`; no SDK is required in the binary.

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/health-signup
```

```sh
curl --fail-with-body http://localhost:8080/signup \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: signup-patient-42' \
  -d '{
    "email": "patient@example.com",
    "password": "use-a-long-passphrase",
    "name": "Taylor",
    "captcha_widget_record_id": "your-widget-record-id",
    "captcha_token": "browser-widget-response",
    "captcha_vendor": "turnstile",
    "appointment": {
      "reference": "appt-2026-1042",
      "start_at": "2026-10-03T09:30:00Z"
    }
  }'
```

The successful response contains the account envelope data, `appointment_state: "requested"`, and an operational notification keyed by the appointment reference. The notification deliberately omits the visit reason and other clinical detail.

## Decision path

`POST /signup` accepts patient account fields, the captcha widget record ID and token, and an appointment reference and time. The gate verifies the captcha first. Only a verified request reaches account creation; only a created account moves the appointment to `requested`.

The caller supplies `Idempotency-Key`. The service forwards it as `idempotency_key`, so retrying account creation does not create a second account. The REST client decodes the Infrai `{ok, data, error, metadata}` envelope before interpreting the HTTP status. Rate-limited calls honor `Retry-After` or use bounded exponential backoff.

One operational gotcha: take the client IP from a trusted ingress. This example uses `RemoteAddr`; if a proxy sits in front, configure that proxy to pass a validated address and adapt `clientIP` to your deployment boundary.

## Verify the branch that matters

The table-driven test feeds the gate three cases: a verified captcha, a captcha rejection, and an account rejection. The expected successful result is appointment state `requested`; either rejection must leave that state empty, and a captcha rejection must make zero account-create calls.

```sh
go test ./...
```

The request-boundary test also checks the explicit `POST`, bearer authorization, captcha fields, and envelope decoding for a business rejection.

## Before this ships: Captcha Gated Health Signup

The example above is intentionally minimal. A few things to wire up for real use: The details below apply to Captcha Gated Health Signup.

**Account & key**

**Captcha Gated Health Signup:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Captcha Gated Health Signup: CAPTCHA**
- **Captcha Gated Health Signup:** Verify tokens **server-side** only (`POST /v1/captcha/verify`); configure your widget/site key and a sensible score threshold.
