package namegender

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func salutationClient(t *testing.T, p *map[string]any, status int, body string) *Client {
	c := New("secret")
	c.BaseURL = "https://example.test"
	c.HTTPClient = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatal("missing auth")
		}
		*p = map[string]any{"_path": r.URL.Path}
		if err := json.NewDecoder(r.Body).Decode(p); err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	return c
}
func TestSalutation(t *testing.T) {
	var p map[string]any
	c := salutationClient(t, &p, 200, `{"credits_charged":1,"credits_remaining":4999,"data_version":"2026.10","request_id":"req_1","country_source":"country","query":"Dr. Anna Müller","language":"de","form":"gendered","reason":null,"salutation":{"formal":"Sehr geehrte Frau Dr. Müller,","informal":"Liebe Anna,","neutral":"Guten Tag Dr. Anna Müller,"},"parts":{"opening":"Sehr geehrte","courtesy":"Frau","academic":"Dr.","name":"Müller"},"gender":"female","gender_source":"lookup","probability":99,"confidence":"high","first_name":"Anna","last_name":"Müller","name_type":"personal","country":"DE"}`)
	r, err := c.Salutation(context.Background(), "Dr. Anna Müller", SalutationOptions{Language: "de", Country: "DE", MinProbability: 95})
	if err != nil {
		t.Fatal(err)
	}
	if p["_path"] != "/salutation" || p["name"] != "Dr. Anna Müller" || p["language"] != "de" || p["country"] != "DE" || p["min_probability"] != float64(95) || len(p) != 5 {
		t.Fatalf("payload=%v", p)
	}
	if r.Salutation.Formal != "Sehr geehrte Frau Dr. Müller," || r.Salutation.Informal != "Liebe Anna," || r.Salutation.Neutral != "Guten Tag Dr. Anna Müller," {
		t.Fatalf("salutation=%+v", r.Salutation)
	}
	if r.Form != "gendered" || r.Reason != nil || r.Parts.Academic == nil || *r.Parts.Academic != "Dr." || r.Probability == nil || *r.Probability != 99 || r.Gender == nil || *r.Gender != "female" || r.NameType != "personal" {
		t.Fatalf("result=%+v", r)
	}
	if r.CreditsCharged != 1 || r.CreditsRemaining != 4999 || r.CountrySource == nil || *r.CountrySource != "country" || r.RequestID == nil || *r.RequestID != "req_1" {
		t.Fatalf("envelope=%+v", r)
	}
}
func TestSalutationNeutral(t *testing.T) {
	var p map[string]any
	c := salutationClient(t, &p, 200, `{"query":"Ahmet Yılmaz","language":"tr","form":"neutral","reason":"gender_neutral_requested","salutation":{"formal":"Sayın Ahmet Yılmaz,","informal":"Merhaba Ahmet,","neutral":"Sayın Ahmet Yılmaz,"},"parts":{"opening":"Sayın","courtesy":null,"academic":null,"name":"Ahmet Yılmaz"},"gender":null,"gender_source":null,"probability":null,"confidence":null,"first_name":"Ahmet","last_name":"Yılmaz","name_type":"personal","country":null,"country_source":null}`)
	r, err := c.Salutation(context.Background(), "", SalutationOptions{FirstName: "Ahmet", LastName: "Yılmaz", Language: "tr", Gender: "neutral", Title: "Prof."})
	if err != nil {
		t.Fatal(err)
	}
	if p["name"] != nil || p["first_name"] != "Ahmet" || p["last_name"] != "Yılmaz" || p["gender"] != "neutral" || p["title"] != "Prof." || len(p) != 6 {
		t.Fatalf("payload=%v", p)
	}
	if r.Form != "neutral" || r.Reason == nil || *r.Reason != "gender_neutral_requested" || r.Parts.Courtesy != nil || r.Parts.Academic != nil || r.Gender != nil || r.Probability != nil || r.Country != nil || r.CountrySource != nil {
		t.Fatalf("result=%+v", r)
	}
	if r.Salutation.Neutral != "Sayın Ahmet Yılmaz," {
		t.Fatalf("salutation=%+v", r.Salutation)
	}
}
func TestSalutationBulk(t *testing.T) {
	var p map[string]any
	c := salutationClient(t, &p, 200, `{"credits_charged":3,"credits_remaining":4996,"data_version":"2026.10","request_id":"req_2","took_ms":4,"country_source":null,"language":"de","summary":{"total":3,"gendered":1,"neutral":1,"organization":1},"results":[{"query":"Dr. Anna Müller","form":"gendered","reason":null,"salutation":{"formal":"Sehr geehrte Frau Dr. Müller,","informal":"Liebe Anna,","neutral":"Guten Tag Dr. Anna Müller,"},"name_type":"personal"},{"query":"Kim Meyer","form":"neutral","reason":"below_min_probability","salutation":{"formal":"Guten Tag Kim Meyer,","informal":"Hallo Kim,","neutral":"Guten Tag Kim Meyer,"},"name_type":"personal"},{"query":"Acme GmbH","form":"organization","reason":null,"salutation":{"formal":"Sehr geehrte Damen und Herren,","informal":"Sehr geehrte Damen und Herren,","neutral":"Sehr geehrte Damen und Herren,"},"name_type":"organization"}]}`)
	names := []string{"Dr. Anna Müller", "Kim Meyer", "Acme GmbH"}
	r, err := c.SalutationBulk(context.Background(), names, SalutationOptions{Language: "de", FirstName: "ignored"})
	if err != nil {
		t.Fatal(err)
	}
	if p["_path"] != "/salutation/bulk" || p["language"] != "de" || p["first_name"] != nil || len(p) != 3 {
		t.Fatalf("payload=%v", p)
	}
	if sent, ok := p["names"].([]any); !ok || len(sent) != 3 || sent[2] != "Acme GmbH" {
		t.Fatalf("names=%v", p["names"])
	}
	if len(r.Results) != 3 || r.Summary != (SalutationSummary{Total: 3, Gendered: 1, Neutral: 1, Organization: 1}) || r.Language != "de" || r.TookMS != 4 || r.CreditsCharged != 3 || r.CountrySource != nil {
		t.Fatalf("result=%+v", r)
	}
	for i, name := range names {
		if r.Results[i].Query != name {
			t.Fatalf("results[%d]=%+v", i, r.Results[i])
		}
	}
	if r.Results[1].Reason == nil || *r.Results[1].Reason != "below_min_probability" || r.Results[2].Form != "organization" || r.Results[2].NameType != "organization" {
		t.Fatalf("results=%+v", r.Results)
	}
}
func TestSalutationUnsupportedLanguage(t *testing.T) {
	var p map[string]any
	c := salutationClient(t, &p, 422, `{"error":"invalid_input","field":"language","message":"Unsupported language.","supported":["en","de","tr"],"request_id":"req_3","docs":"x"}`)
	_, err := c.Salutation(context.Background(), "Anna", SalutationOptions{Language: "xx"})
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Status != 422 || !strings.Contains(string(apiErr.Body), `"field":"language"`) {
		t.Fatalf("err=%v", err)
	}
}
