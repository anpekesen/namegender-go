package namegender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Client struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// Options configures Name, Email, Username and Bulk. AIFallback falls back to a
// language model for names not in the database and needs AI consent on the
// account. BestGuess returns the most likely gender even below the probability
// threshold.
type Options struct {
	Country    string `json:"country,omitempty"`
	AIFallback bool   `json:"ai_fallback,omitempty"`
	BestGuess  bool   `json:"best_guess,omitempty"`
	Type       string `json:"type,omitempty"`
}

// Result is one lookup. Success is carried by the HTTP status: a non-2xx
// response is returned as an *APIError, never as a Result.
type Result struct {
	Query            string  `json:"query"`
	Name             string  `json:"name"`
	Gender           *string `json:"gender"`
	Country          *string `json:"country"`
	Probability      int     `json:"probability"`
	SampleSize       int     `json:"sample_size"`
	TookMS           int     `json:"took_ms"`
	Confidence       string  `json:"confidence"`
	Source           string  `json:"source"`
	MatchedAs        *string `json:"matched_as"`
	FirstName        *string `json:"first_name"`
	MiddleName       *string `json:"middle_name"`
	LastName         *string `json:"last_name"`
	CreditsCharged   int     `json:"credits_charged"`
	CreditsRemaining int     `json:"credits_remaining"`
	DataVersion      *string `json:"data_version"`
	RequestID        *string `json:"request_id"`
}
type BulkResult struct {
	Results          []Result       `json:"results"`
	Summary          map[string]any `json:"summary"`
	TookMS           int            `json:"took_ms"`
	CreditsCharged   int            `json:"credits_charged"`
	CreditsRemaining int            `json:"credits_remaining"`
	DataVersion      *string        `json:"data_version"`
	RequestID        *string        `json:"request_id"`
}

// CountriesOptions configures Countries. Limit caps the number of entries in
// Registrations (1-100); zero leaves the server default of 25.
type CountriesOptions struct {
	Limit int `json:"limit,omitempty"`
}

// CountriesResult is the country distribution of a name. It is not a
// country-of-origin or ethnicity inference: Registrations is counted volume,
// comparable only among countries that publish counted birth statistics, and
// AttestedIn is presence with no weight attached. Show Basis.Note next to any
// percentage.
type CountriesResult struct {
	Name          string                `json:"name"`
	Basis         CountriesBasis        `json:"basis"`
	Registrations []CountryRegistration `json:"registrations"`
	AttestedIn    []string              `json:"attested_in"`

	TookMS           int     `json:"took_ms"`
	CreditsCharged   int     `json:"credits_charged"`
	CreditsRemaining int     `json:"credits_remaining"`
	DataVersion      *string `json:"data_version"`
	RequestID        *string `json:"request_id"`
}

// CountriesBasis states what the numbers in a CountriesResult rest on.
type CountriesBasis struct {
	CountedSources    []string `json:"counted_sources"`
	CountedCountries  int      `json:"counted_countries"`
	AttestedCountries int      `json:"attested_countries"`
	Note              string   `json:"note"`
}

// CountryRegistration is one counted country. Share is a percentage of the
// registrations in this list alone.
type CountryRegistration struct {
	Country     string  `json:"country"`
	Count       int     `json:"count"`
	Share       float64 `json:"share"`
	Gender      *string `json:"gender"`
	Probability int     `json:"probability"`
	Source      string  `json:"source"`
}
type APIError struct {
	Status int
	Body   []byte
}

func (e *APIError) Error() string { return fmt.Sprintf("namegender: HTTP %d: %s", e.Status, e.Body) }

func New(apiKey string) *Client {
	return &Client{APIKey: apiKey, BaseURL: "https://namegender.com/api/v1", HTTPClient: http.DefaultClient}
}
func (c *Client) Name(ctx context.Context, name string, o Options) (*Result, error) {
	return c.single(ctx, "/gender", "name", name, o)
}
func (c *Client) Email(ctx context.Context, email string, o Options) (*Result, error) {
	return c.single(ctx, "/gender/email", "email", email, o)
}
func (c *Client) Username(ctx context.Context, username string, o Options) (*Result, error) {
	return c.single(ctx, "/gender/username", "username", username, o)
}
func (c *Client) single(ctx context.Context, path, field, value string, o Options) (*Result, error) {
	p := map[string]any{field: value}
	applyOptions(p, o)
	var result Result
	return &result, c.request(ctx, path, p, &result)
}
func (c *Client) Bulk(ctx context.Context, names []string, o Options) (*BulkResult, error) {
	p := map[string]any{"names": names}
	applyOptions(p, o)
	var result BulkResult
	return &result, c.request(ctx, "/gender/bulk", p, &result)
}
func (c *Client) Countries(ctx context.Context, name string, o CountriesOptions) (*CountriesResult, error) {
	p := map[string]any{"name": name}
	if o.Limit > 0 {
		p["limit"] = o.Limit
	}
	var result CountriesResult
	return &result, c.request(ctx, "/gender/countries", p, &result)
}
func applyOptions(p map[string]any, o Options) {
	if o.Country != "" {
		p["country"] = o.Country
	}
	if o.AIFallback {
		p["ai_fallback"] = true
	}
	if o.BestGuess {
		p["best_guess"] = true
	}
	if o.Type != "" {
		p["type"] = o.Type
	}
}
func (c *Client) request(ctx context.Context, path string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	raw, err := c.send(ctx, http.MethodPost, path, "application/json", body, nil)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

// send makes one request and returns the response body. A non-2xx response is
// an *APIError. body may be nil; it is sent as given, so the caller can send
// the same bytes again on a retry.
func (c *Client) send(ctx context.Context, method, path, contentType string, body []byte, header http.Header) ([]byte, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("namegender: API key is required")
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, reader)
	if err != nil {
		return nil, err
	}
	for key, values := range header {
		req.Header[key] = values
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{Status: resp.StatusCode, Body: raw}
	}
	return raw, nil
}
