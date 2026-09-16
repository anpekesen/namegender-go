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
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":true,"gender":"female"}`)), Header: make(http.Header)}, nil
	})}
	r, err := c.Name(context.Background(), "Ayşe", Options{Country: "TR"})
	if err != nil || r.Gender == nil || *r.Gender != "female" {
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
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":true,"name":"Mehmet","basis":{"counted_sources":["insee","ons"],"counted_countries":2,"attested_countries":3,"note":"n"},"registrations":[{"country":"FR","count":3775,"share":58.97,"gender":"male","probability":99,"source":"insee"},{"country":"GB","count":1130,"share":17.65,"gender":null,"probability":0,"source":"ons"}],"attested_in":["FR","GB","TR"]}`)), Header: make(http.Header)}, nil
	})}
	r, err := c.Countries(context.Background(), "Mehmet", CountriesOptions{Limit: 10})
	if err != nil || len(r.Registrations) != 2 || r.Registrations[0].Share != 58.97 || r.Registrations[0].Gender == nil || r.Registrations[1].Gender != nil || r.Basis.AttestedCountries != 3 || len(r.AttestedIn) != 3 {
		t.Fatalf("result=%+v err=%v", r, err)
	}
}
