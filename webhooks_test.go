package namegender

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// The shared vector: every NameGender SDK must accept exactly this.
const (
	vectorSecret    = "whsec_test_vector"
	vectorBody      = `{"id":"evt_1","type":"webhook.test"}`
	vectorSignature = "857fcddfea47617c448b7a8e6537bbd59c9922a37c5273b2709812fbadb29e50"
)

var vectorNow = WithNow(func() time.Time { return time.Unix(1700000000, 0) })

func TestVerifyWebhookVector(t *testing.T) {
	for _, header := range []string{
		"t=1700000000,v1=" + vectorSignature,
		"t=1700000000,v1=" + strings.Repeat("0", 64) + ",v1=" + vectorSignature,
	} {
		event, err := VerifyWebhook([]byte(vectorBody), header, vectorSecret, vectorNow)
		if err != nil || event.ID != "evt_1" || event.Type != "webhook.test" {
			t.Fatalf("%s: event=%+v err=%v", header, event, err)
		}
	}
}

func TestVerifyWebhookRejects(t *testing.T) {
	header := "t=1700000000,v1=" + vectorSignature
	cases := map[string]struct {
		body, header, secret string
		opts                 []VerifyOption
	}{
		"tampered body":  {`{"id":"evt_2","type":"webhook.test"}`, header, vectorSecret, []VerifyOption{vectorNow}},
		"wrong secret":   {vectorBody, header, "whsec_other", []VerifyOption{vectorNow}},
		"stale":          {vectorBody, header, vectorSecret, []VerifyOption{WithNow(func() time.Time { return time.Unix(1700000301, 0) })}},
		"future":         {vectorBody, header, vectorSecret, []VerifyOption{WithNow(func() time.Time { return time.Unix(1699999699, 0) })}},
		"missing header": {vectorBody, "", vectorSecret, []VerifyOption{vectorNow}},
		"malformed":      {vectorBody, "t=abc,v1=", vectorSecret, []VerifyOption{vectorNow}},
		"no v1":          {vectorBody, "t=1700000000", vectorSecret, []VerifyOption{vectorNow}},
	}
	for name, tc := range cases {
		_, err := VerifyWebhook([]byte(tc.body), tc.header, tc.secret, tc.opts...)
		if !errors.Is(err, ErrWebhookSignature) {
			t.Errorf("%s: err=%v", name, err)
		}
	}
	if _, err := VerifyWebhook([]byte(vectorBody), header, vectorSecret, WithNow(func() time.Time { return time.Unix(1700000300, 0) })); err != nil {
		t.Errorf("edge of tolerance rejected: %v", err)
	}
	if _, err := VerifyWebhook([]byte(vectorBody), header, vectorSecret, WithTolerance(time.Hour), WithNow(func() time.Time { return time.Unix(1700003000, 0) })); err != nil {
		t.Errorf("wider tolerance rejected: %v", err)
	}
}

func TestWebhookEventObjects(t *testing.T) {
	batch := WebhookEvent{Type: "batch.completed", Data: WebhookEventData{Object: []byte(`{"id":"B-1","status":"completed","summary":{"male":1,"female":2,"unknown":0,"from_llm":0},"result":{"url":"https://x/r","format":"csv","expires_at":null}}`)}}
	job, err := batch.Batch()
	if err != nil || job.ID != "B-1" || job.Summary.Male != 1 || job.Result.Format != "csv" || job.Result.ExpiresAt != nil {
		t.Fatalf("job=%+v err=%v", job, err)
	}
	if _, err := batch.CreditsAlert(); err == nil {
		t.Fatal("batch event read as credits alert")
	}
	low := WebhookEvent{Type: "credits.low", Data: WebhookEventData{Object: []byte(`{"kind":"credits_low","credits_remaining":120,"purchased":100,"subscription":20,"daily_burn":40,"runway_days":3,"since":"2026-09-24T10:00:00Z"}`)}}
	alert, err := low.CreditsAlert()
	if err != nil || alert.Kind != "credits_low" || alert.CreditsRemaining != 120 || alert.RunwayDays == nil || *alert.RunwayDays != 3 || alert.Since.Hour() != 10 {
		t.Fatalf("alert=%+v err=%v", alert, err)
	}
	if _, err := low.Batch(); err == nil {
		t.Fatal("credits event read as job")
	}
}
