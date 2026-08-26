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
