package namegender

import (
	"context"
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
