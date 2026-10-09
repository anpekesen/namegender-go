package namegender

import (
	"context"
	"strings"
	"testing"
)

func TestAge(t *testing.T) {
	var p map[string]any
	c := salutationClient(t, &p, 200, `{"credits_charged":1,"credits_remaining":49999,"request_id":"req_1","name":"Brittany","first_name":"Brittany","gender":null,"age":36,"age_range":{"low":32,"high":38},"age_range_80":{"low":28,"high":41},"birth_year":1990,"sample_size":353775,"births":361434,"country":"US","country_source":"default","source":"ssa","series":"1880-2024","reference_year":2026,"reason":null}`)
	r, err := c.Age(context.Background(), "Brittany", AgeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if p["_path"] != "/age" || p["name"] != "Brittany" || len(p) != 2 {
		t.Fatalf("payload=%v", p)
	}
	if r.Name != "Brittany" || r.FirstName == nil || *r.FirstName != "Brittany" || r.Gender != nil || r.Age == nil || *r.Age != 36 || r.BirthYear == nil || *r.BirthYear != 1990 || r.Reason != nil {
		t.Fatalf("result=%+v", r)
	}
	if r.AgeRange == nil || *r.AgeRange != (AgeRange{Low: 32, High: 38}) || r.AgeRange80 == nil || *r.AgeRange80 != (AgeRange{Low: 28, High: 41}) {
		t.Fatalf("ranges=%+v %+v", r.AgeRange, r.AgeRange80)
	}
	if r.SampleSize != 353775 || r.Births != 361434 || r.Country != "US" || r.CountrySource != "default" || r.Source == nil || *r.Source != "ssa" || r.Series == nil || *r.Series != "1880-2024" || r.ReferenceYear != 2026 {
		t.Fatalf("result=%+v", r)
	}
	if r.CreditsCharged != 1 || r.CreditsRemaining != 49999 || r.RequestID == nil || *r.RequestID != "req_1" {
		t.Fatalf("envelope=%+v", r)
	}
}
func TestAgeWithOptions(t *testing.T) {
	var p map[string]any
	c := salutationClient(t, &p, 200, `{"credits_charged":1,"credits_remaining":49998,"request_id":"req_2","name":"Camille","first_name":"Camille","gender":"female","age":30,"age_range":{"low":24,"high":39},"age_range_80":{"low":19,"high":52},"birth_year":1996,"sample_size":120000,"births":125000,"country":"FR","country_source":"country","source":"insee","series":"1900-2023","reference_year":2026,"reason":null}`)
	r, err := c.Age(context.Background(), "Camille", AgeOptions{Gender: "female", Country: "FR", Locale: "fr-FR", IP: "203.0.113.7"})
	if err != nil {
		t.Fatal(err)
	}
	if p["gender"] != "female" || p["country"] != "FR" || p["locale"] != "fr-FR" || p["ip"] != "203.0.113.7" || len(p) != 6 {
		t.Fatalf("payload=%v", p)
	}
	if r.Gender == nil || *r.Gender != "female" || r.CountrySource != "country" {
		t.Fatalf("result=%+v", r)
	}
}
func TestAgeCountryNotCovered(t *testing.T) {
	var p map[string]any
	c := salutationClient(t, &p, 200, `{"credits_charged":0,"credits_remaining":49999,"request_id":"req_3","name":"Anna","first_name":"Anna","gender":null,"age":null,"age_range":null,"age_range_80":null,"birth_year":null,"sample_size":0,"births":0,"country":"DE","country_source":"country","source":null,"series":null,"reference_year":2026,"reason":"country_not_covered"}`)
	r, err := c.Age(context.Background(), "Anna", AgeOptions{Country: "DE"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Age != nil || r.AgeRange != nil || r.AgeRange80 != nil || r.BirthYear != nil || r.Source != nil || r.Series != nil || r.Reason == nil || *r.Reason != "country_not_covered" || r.CreditsCharged != 0 || r.Country != "DE" {
		t.Fatalf("result=%+v", r)
	}
}
func TestAgeBulk(t *testing.T) {
	var p map[string]any
	c := salutationClient(t, &p, 200, `{"credits_charged":2,"credits_remaining":49997,"request_id":"req_4","country_source":"locale","results":[{"name":"Brittany","first_name":"Brittany","gender":"female","age":36,"age_range":{"low":32,"high":38},"age_range_80":{"low":28,"high":41},"birth_year":1990,"sample_size":353000,"births":360000,"country":"US","country_source":"locale","source":"ssa","series":"1880-2024","reference_year":2026,"reason":null},{"name":"Xqzv","first_name":null,"gender":"female","age":null,"age_range":null,"age_range_80":null,"birth_year":null,"sample_size":0,"births":0,"country":"US","country_source":"locale","source":null,"series":null,"reference_year":2026,"reason":"not_found"}]}`)
	names := []string{"Brittany", "Xqzv"}
	r, err := c.AgeBulk(context.Background(), names, AgeOptions{Gender: "female", Locale: "en-US"})
	if err != nil {
		t.Fatal(err)
	}
	if p["_path"] != "/age/bulk" || p["gender"] != "female" || p["locale"] != "en-US" || len(p) != 4 {
		t.Fatalf("payload=%v", p)
	}
	if sent, ok := p["names"].([]any); !ok || len(sent) != 2 || sent[1] != "Xqzv" {
		t.Fatalf("names=%v", p["names"])
	}
	if len(r.Results) != 2 || r.CreditsCharged != 2 || r.CreditsRemaining != 49997 || r.CountrySource != "locale" || r.RequestID == nil || *r.RequestID != "req_4" {
		t.Fatalf("result=%+v", r)
	}
	for i, name := range names {
		if r.Results[i].Name != name {
			t.Fatalf("results[%d]=%+v", i, r.Results[i])
		}
	}
	if a := r.Results[0]; a.Age == nil || *a.Age != 36 || a.AgeRange80 == nil || a.AgeRange80.High != 41 {
		t.Fatalf("results[0]=%+v", a)
	}
	if a := r.Results[1]; a.Age != nil || a.FirstName != nil || a.Reason == nil || *a.Reason != "not_found" {
		t.Fatalf("results[1]=%+v", a)
	}
}
func TestAgeNoCredits(t *testing.T) {
	var p map[string]any
	c := salutationClient(t, &p, 402, `{"error":"no_credits","message":"No credits left.","request_id":"req_5","docs":"x"}`)
	_, err := c.Age(context.Background(), "Anna", AgeOptions{})
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Status != 402 || !strings.Contains(string(apiErr.Body), `"error":"no_credits"`) {
		t.Fatalf("err=%v", err)
	}
	if len(p) != 2 || p["name"] != "Anna" {
		t.Fatalf("payload=%v", p)
	}
}
