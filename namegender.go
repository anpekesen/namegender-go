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
type Options struct {
	Country string `json:"country,omitempty"`
	AskToAI bool   `json:"askToAI,omitempty"`
	Force   bool   `json:"forceToGenderize,omitempty"`
	Type    string `json:"type,omitempty"`
}
type Result struct {
	Status      bool    `json:"status"`
	Name        string  `json:"name"`
	Gender      *string `json:"gender"`
	Country     *string `json:"country"`
	Probability int     `json:"probability"`
	TotalNames  int     `json:"total_names"`
	Confidence  string  `json:"confidence"`
	Source      string  `json:"source"`
}
type BulkResult struct {
	Status  bool           `json:"status"`
	Results []Result       `json:"results"`
	Summary map[string]any `json:"summary"`
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
	Status        bool                  `json:"status"`
	Name          string                `json:"name"`
	Basis         CountriesBasis        `json:"basis"`
	Registrations []CountryRegistration `json:"registrations"`
	AttestedIn    []string              `json:"attested_in"`
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
	if o.AskToAI {
		p["askToAI"] = true
	}
	if o.Force {
		p["forceToGenderize"] = true
	}
	if o.Type != "" {
		p["type"] = o.Type
	}
}
func (c *Client) request(ctx context.Context, path string, payload any, target any) error {
	if c.APIKey == "" {
		return fmt.Errorf("namegender: API key is required")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{Status: resp.StatusCode, Body: raw}
	}
	return json.Unmarshal(raw, target)
}
