package namegender

import (
	"context"
	"strings"
	"testing"
)

func TestNameCheck(t *testing.T) {
	var p map[string]any
	c := salutationClient(t, &p, 200, `{"credits_charged":1,"credits_remaining":4999,"data_version":"2026.10","request_id":"req_1","country_source":"country","query":"asdf qwerty","assessment":"implausible","score":0,"signals":[{"code":"keyboard_pattern","severity":"high","part":"first_name","value":"asdf"},{"code":"keyboard_pattern","severity":"high","part":"last_name","value":"qwerty"},{"code":"first_name_not_found","severity":"medium","part":null,"value":null}],"first_name":"Asdf","last_name":"Qwerty","name_type":"personal","evidence":{"first_name_status":"not_found","first_name_counted_records":0}}`)
	r, err := c.NameCheck(context.Background(), "asdf qwerty", NameCheckOptions{Country: "US"})
	if err != nil {
		t.Fatal(err)
	}
	if p["_path"] != "/name-check" || p["name"] != "asdf qwerty" || p["country"] != "US" || len(p) != 3 {
		t.Fatalf("payload=%v", p)
	}
	if r.Assessment != "implausible" || r.Score != 0 || len(r.Signals) != 3 || r.NameType != "personal" || r.FirstName == nil || *r.FirstName != "Asdf" || r.LastName == nil || *r.LastName != "Qwerty" {
		t.Fatalf("result=%+v", r)
	}
	s := r.Signals[0]
	if s.Code != "keyboard_pattern" || s.Severity != "high" || s.Part == nil || *s.Part != "first_name" || s.Value == nil || *s.Value != "asdf" {
		t.Fatalf("signal=%+v", s)
	}
	if s = r.Signals[2]; s.Code != "first_name_not_found" || s.Part != nil || s.Value != nil {
		t.Fatalf("signal=%+v", s)
	}
	if r.Evidence.FirstNameStatus == nil || *r.Evidence.FirstNameStatus != "not_found" || r.Evidence.FirstNameCountedRecords != 0 {
		t.Fatalf("evidence=%+v", r.Evidence)
	}
	if r.CreditsCharged != 1 || r.CreditsRemaining != 4999 || r.CountrySource == nil || *r.CountrySource != "country" || r.RequestID == nil || *r.RequestID != "req_1" {
		t.Fatalf("envelope=%+v", r)
	}
}
func TestNameCheckByParts(t *testing.T) {
	var p map[string]any
	c := salutationClient(t, &p, 200, `{"query":"Jennifer Null","country_source":null,"assessment":"plausible","score":96,"signals":[{"code":"first_name_attested","severity":"positive","part":"first_name","value":"Jennifer"}],"first_name":"Jennifer","last_name":"Null","name_type":"personal","evidence":{"first_name_status":null,"first_name_counted_records":5871000}}`)
	r, err := c.NameCheck(context.Background(), "", NameCheckOptions{FirstName: "Jennifer", LastName: "Null", Locale: "en-US"})
	if err != nil {
		t.Fatal(err)
	}
	if p["name"] != nil || p["first_name"] != "Jennifer" || p["last_name"] != "Null" || p["locale"] != "en-US" || len(p) != 4 {
		t.Fatalf("payload=%v", p)
	}
	if r.Assessment != "plausible" || r.Score != 96 || r.CountrySource != nil || r.Evidence.FirstNameStatus != nil || r.Evidence.FirstNameCountedRecords != 5871000 {
		t.Fatalf("result=%+v", r)
	}
	if len(r.Signals) != 1 || r.Signals[0].Severity != "positive" {
		t.Fatalf("signals=%+v", r.Signals)
	}
}
func TestNameCheckBulk(t *testing.T) {
	var p map[string]any
	c := salutationClient(t, &p, 200, `{"credits_charged":3,"credits_remaining":4996,"data_version":"2026.10","request_id":"req_2","took_ms":4,"country_source":"ip","summary":{"total":3,"plausible":1,"suspicious":1,"implausible":1},"results":[{"query":"Jennifer Null","assessment":"plausible","score":96,"signals":[],"first_name":"Jennifer","last_name":"Null","name_type":"personal","evidence":{"first_name_status":"counted","first_name_counted_records":5871000}},{"query":"Mickey Mouse","assessment":"suspicious","score":35,"signals":[{"code":"fictional_character","severity":"medium","part":"full","value":"Mickey Mouse"}],"first_name":"Mickey","last_name":"Mouse","name_type":"personal","evidence":{"first_name_status":"counted","first_name_counted_records":1200}},{"query":"asdf qwerty","assessment":"implausible","score":0,"signals":[{"code":"keyboard_pattern","severity":"high","part":"first_name","value":"asdf"}],"first_name":"Asdf","last_name":"Qwerty","name_type":"personal","evidence":{"first_name_status":"not_found","first_name_counted_records":0}}]}`)
	names := []string{"Jennifer Null", "Mickey Mouse", "asdf qwerty"}
	r, err := c.NameCheckBulk(context.Background(), names, NameCheckOptions{IP: "203.0.113.7", FirstName: "ignored"})
	if err != nil {
		t.Fatal(err)
	}
	if p["_path"] != "/name-check/bulk" || p["ip"] != "203.0.113.7" || p["first_name"] != nil || len(p) != 3 {
		t.Fatalf("payload=%v", p)
	}
	if sent, ok := p["names"].([]any); !ok || len(sent) != 3 || sent[2] != "asdf qwerty" {
		t.Fatalf("names=%v", p["names"])
	}
	if len(r.Results) != 3 || r.Summary != (NameCheckSummary{Total: 3, Plausible: 1, Suspicious: 1, Implausible: 1}) || r.TookMS != 4 || r.CreditsCharged != 3 || r.CountrySource == nil || *r.CountrySource != "ip" {
		t.Fatalf("result=%+v", r)
	}
	want := []string{"plausible", "suspicious", "implausible"}
	for i, name := range names {
		if r.Results[i].Query != name || r.Results[i].Assessment != want[i] {
			t.Fatalf("results[%d]=%+v", i, r.Results[i])
		}
	}
	if s := r.Results[1].Signals[0]; s.Code != "fictional_character" || s.Part == nil || *s.Part != "full" {
		t.Fatalf("signal=%+v", s)
	}
}
func TestNameCheckNoCredits(t *testing.T) {
	var p map[string]any
	c := salutationClient(t, &p, 402, `{"error":"no_credits","message":"No credits left.","request_id":"req_3","docs":"x"}`)
	_, err := c.NameCheck(context.Background(), "Anna", NameCheckOptions{})
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Status != 402 || !strings.Contains(string(apiErr.Body), `"error":"no_credits"`) {
		t.Fatalf("err=%v", err)
	}
	if len(p) != 2 || p["name"] != "Anna" {
		t.Fatalf("payload=%v", p)
	}
}
