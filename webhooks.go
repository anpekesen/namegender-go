package namegender

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// WebhookEvent is the envelope of every webhook request. Type is one of
// batch.completed, batch.failed, credits.low, credits.depleted and
// webhook.test; new types can be added, so answer 2xx to one you do not
// recognise and ignore it. ID is the same on every retry: deduplicate on it.
type WebhookEvent struct {
	ID         string           `json:"id"`
	Type       string           `json:"type"`
	CreatedAt  time.Time        `json:"created_at"`
	APIVersion string           `json:"api_version"`
	Data       WebhookEventData `json:"data"`
}

// WebhookEventData holds the object the event is about, undecoded. Use
// WebhookEvent.Batch or WebhookEvent.CreditsAlert to read it.
type WebhookEventData struct {
	Object json.RawMessage `json:"object"`
}

// CreditsAlert is data.object of credits.low and credits.depleted.
// CreditsRemaining is the same number as credits_remaining on /me: purchased,
// subscription and today's free credits together. RunwayDays is set for
// credits.low only.
type CreditsAlert struct {
	Kind             string    `json:"kind"` // credits_low or credits_out
	CreditsRemaining int       `json:"credits_remaining"`
	Purchased        int       `json:"purchased"`
	Subscription     int       `json:"subscription"`
	DailyBurn        int       `json:"daily_burn"`
	RunwayDays       *int      `json:"runway_days"`
	Since            time.Time `json:"since"`
}

// Batch decodes data.object of batch.completed and batch.failed: the job, as
// GetBatch returns it.
func (e *WebhookEvent) Batch() (*BatchJob, error) {
	if !strings.HasPrefix(e.Type, "batch.") {
		return nil, fmt.Errorf("namegender: %s event does not carry a file job", e.Type)
	}
	var job BatchJob
	return &job, json.Unmarshal(e.Data.Object, &job)
}

// CreditsAlert decodes data.object of credits.low and credits.depleted.
func (e *WebhookEvent) CreditsAlert() (*CreditsAlert, error) {
	if !strings.HasPrefix(e.Type, "credits.") {
		return nil, fmt.Errorf("namegender: %s event does not carry a credits alert", e.Type)
	}
	var alert CreditsAlert
	return &alert, json.Unmarshal(e.Data.Object, &alert)
}

// ErrWebhookSignature is wrapped by every error VerifyWebhook returns for a
// request that is not a genuine NameGender webhook. Answer it with 400.
var ErrWebhookSignature = errors.New("namegender: webhook signature")

type webhookError struct{ msg string }

func (e *webhookError) Error() string { return "namegender: " + e.msg }
func (e *webhookError) Unwrap() error { return ErrWebhookSignature }

// VerifyOption adjusts VerifyWebhook.
type VerifyOption func(*verifyConfig)

type verifyConfig struct {
	tolerance time.Duration
	now       func() time.Time
}

// WithTolerance sets how far the signed timestamp may be from the clock
// (default 5 minutes).
func WithTolerance(d time.Duration) VerifyOption {
	return func(c *verifyConfig) { c.tolerance = d }
}

// WithNow replaces the clock, for tests.
func WithNow(now func() time.Time) VerifyOption {
	return func(c *verifyConfig) { c.now = now }
}

// VerifyWebhook checks the NameGender-Signature header and returns the parsed
// event.
//
// payload must be the body exactly as received (io.ReadAll(r.Body) before any
// JSON decoding): decoding and encoding again changes the bytes, and the
// signature no longer matches. During a secret rotation the header carries two
// v1 values; either one matching is enough. An error wrapping
// ErrWebhookSignature means the request is not genuine.
func VerifyWebhook(payload []byte, signatureHeader, secret string, opts ...VerifyOption) (*WebhookEvent, error) {
	if secret == "" {
		return nil, errors.New("namegender: webhook secret is required")
	}
	cfg := verifyConfig{tolerance: 300 * time.Second, now: time.Now}
	for _, opt := range opts {
		opt(&cfg)
	}
	if signatureHeader == "" {
		return nil, &webhookError{"missing NameGender-Signature header"}
	}

	var timestamp int64
	haveTimestamp := false
	var signatures []string
	for _, part := range strings.Split(signatureHeader, ",") {
		key, value, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch {
		case key == "t" && isDigits(value):
			if t, err := strconv.ParseInt(value, 10, 64); err == nil {
				timestamp, haveTimestamp = t, true
			}
		case key == "v1" && value != "":
			signatures = append(signatures, value)
		}
	}
	if !haveTimestamp || len(signatures) == 0 {
		return nil, &webhookError{"malformed NameGender-Signature header"}
	}

	age := cfg.now().Sub(time.Unix(timestamp, 0))
	if age > cfg.tolerance || -age > cfg.tolerance {
		return nil, &webhookError{"webhook timestamp is outside the tolerance window"}
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10) + "."))
	mac.Write(payload)
	expected := []byte(hex.EncodeToString(mac.Sum(nil)))

	matched := false
	for _, signature := range signatures {
		if hmac.Equal(expected, []byte(signature)) {
			matched = true
		}
	}
	if !matched {
		return nil, &webhookError{"webhook signature does not match"}
	}

	var event WebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
