package namegender

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestName(t *testing.T) {
	c := New("secret")
	c.BaseURL = "https://example.test"
	c.HTTPClient = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatal("missing auth")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"query":"Ayşe","gender":"female","sample_size":12345,"took_ms":4,"credits_remaining":9}`)), Header: make(http.Header)}, nil
	})}
	r, err := c.Name(context.Background(), "Ayşe", Options{Country: "TR"})
	if err != nil || r.Gender == nil || *r.Gender != "female" || r.Query != "Ayşe" || r.SampleSize != 12345 || r.TookMS != 4 || r.CreditsRemaining != 9 {
		t.Fatalf("result=%+v err=%v", r, err)
	}
}
func TestCountries(t *testing.T) {
	c := New("secret")
	c.BaseURL = "https://example.test"
	c.HTTPClient = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatal("missing auth")
		}
		if r.URL.Path != "/gender/countries" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		var p map[string]any
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p["name"] != "Mehmet" || p["limit"] != float64(10) {
			t.Fatalf("payload=%v err=%v", p, err)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"took_ms":12,"name":"Mehmet","basis":{"counted_sources":["insee","ons"],"counted_countries":2,"attested_countries":3,"note":"n"},"registrations":[{"country":"FR","count":3775,"share":58.97,"gender":"male","probability":99,"source":"insee"},{"country":"GB","count":1130,"share":17.65,"gender":null,"probability":0,"source":"ons"}],"attested_in":["FR","GB","TR"]}`)), Header: make(http.Header)}, nil
	})}
	r, err := c.Countries(context.Background(), "Mehmet", CountriesOptions{Limit: 10})
	if err != nil || len(r.Registrations) != 2 || r.Registrations[0].Share != 58.97 || r.Registrations[0].Gender == nil || r.Registrations[1].Gender != nil || r.Basis.AttestedCountries != 3 || len(r.AttestedIn) != 3 || r.TookMS != 12 {
		t.Fatalf("result=%+v err=%v", r, err)
	}
}
func TestOptions(t *testing.T) {
	c := New("secret")
	c.BaseURL = "https://example.test"
	c.HTTPClient = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		var p map[string]any
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p["ai_fallback"] != true || p["best_guess"] != true || p["country"] != "IT" || len(p) != 4 {
			t.Fatalf("payload=%v err=%v", p, err)
		}
		return &http.Response{StatusCode: 402, Body: io.NopCloser(strings.NewReader(`{"error":"no_credits","message":"Out of credits.","request_id":"req_1","docs":"x"}`)), Header: make(http.Header)}, nil
	})}
	_, err := c.Name(context.Background(), "Andrea", Options{Country: "IT", AIFallback: true, BestGuess: true})
	if apiErr, ok := err.(*APIError); !ok || apiErr.Status != 402 {
		t.Fatalf("err=%v", err)
	}
}
