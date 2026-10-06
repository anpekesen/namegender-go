package namegender

import "context"

// SalutationOptions configures Salutation and SalutationBulk. Empty fields are
// not sent.
//
// FirstName and LastName replace the name when the two are stored separately;
// they are not parsed. SalutationBulk ignores them.
//
// Language is the language of the salutation (en, en-US, en-GB, de, de-AT,
// de-CH, fr, es, it, pt, pt-PT, pt-BR, nl, tr, pl, ja). Without it the API uses
// the language of Locale, else the main language of the country, else English.
// An unsupported language is a 422 *APIError.
//
// Country, Locale and IP are the same country hints as in Options. Gender
// ("male", "female" or "neutral") is a gender you already know and overrides
// the lookup. MinProbability (50-100, zero leaves the server default of 90)
// is the lowest probability for a gendered salutation. Title is an academic
// title kept in a separate field ("Dr.").
type SalutationOptions struct {
	FirstName      string
	LastName       string
	Language       string
	Country        string
	Locale         string
	IP             string
	Gender         string
	MinProbability int
	Title          string
}

// SalutationItem is the salutation for one name. Nullable fields are pointers.
type SalutationItem struct {
	Query        string          `json:"query"`
	Language     string          `json:"language"`
	Form         string          `json:"form"`   // gendered, neutral or organization
	Reason       *string         `json:"reason"` // why the neutral form was used; nil otherwise
	Salutation   SalutationLines `json:"salutation"`
	Parts        SalutationParts `json:"parts"`
	Gender       *string         `json:"gender"`        // male, female or nil
	GenderSource *string         `json:"gender_source"` // lookup, input, title or nil
	Probability  *int            `json:"probability"`
	Confidence   *string         `json:"confidence"`
	FirstName    *string         `json:"first_name"`
	LastName     *string         `json:"last_name"`
	NameType     string          `json:"name_type"` // personal, organization or role
	Country      *string         `json:"country"`
}

// SalutationLines holds the three ready lines. Neutral is always the
// gender-free form, for applying your own threshold.
type SalutationLines struct {
	Formal   string `json:"formal"`
	Informal string `json:"informal"`
	Neutral  string `json:"neutral"`
}

// SalutationParts are the pieces of the formal line, for building your own
// template.
type SalutationParts struct {
	Opening  *string `json:"opening"`
	Courtesy *string `json:"courtesy"`
	Academic *string `json:"academic"`
	Name     *string `json:"name"`
}

// SalutationResult is the response of Salutation.
type SalutationResult struct {
	SalutationItem
	CountrySource    *string `json:"country_source"` // country, locale, ip or nil
	CreditsCharged   int     `json:"credits_charged"`
	CreditsRemaining int     `json:"credits_remaining"`
	DataVersion      *string `json:"data_version"`
	RequestID        *string `json:"request_id"`
}

// SalutationBulkResult is the response of SalutationBulk. Results are in
// input order.
type SalutationBulkResult struct {
	Results          []SalutationItem  `json:"results"`
	Summary          SalutationSummary `json:"summary"`
	Language         string            `json:"language"`
	CountrySource    *string           `json:"country_source"` // country, locale, ip or nil
	TookMS           int               `json:"took_ms"`
	CreditsCharged   int               `json:"credits_charged"`
	CreditsRemaining int               `json:"credits_remaining"`
	DataVersion      *string           `json:"data_version"`
	RequestID        *string           `json:"request_id"`
}

// SalutationSummary counts the forms in a SalutationBulkResult.
type SalutationSummary struct {
	Total        int `json:"total"`
	Gendered     int `json:"gendered"`
	Neutral      int `json:"neutral"`
	Organization int `json:"organization"`
}

// Salutation returns the letter salutation for one name. name may be empty
// when FirstName or LastName is set. One credit.
func (c *Client) Salutation(ctx context.Context, name string, o SalutationOptions) (*SalutationResult, error) {
	p := map[string]any{}
	if name != "" {
		p["name"] = name
	}
	if o.FirstName != "" {
		p["first_name"] = o.FirstName
	}
	if o.LastName != "" {
		p["last_name"] = o.LastName
	}
	applySalutationOptions(p, o)
	var result SalutationResult
	return &result, c.request(ctx, "/salutation", p, &result)
}

// SalutationBulk returns the salutations for up to 100 names, one credit each.
func (c *Client) SalutationBulk(ctx context.Context, names []string, o SalutationOptions) (*SalutationBulkResult, error) {
	p := map[string]any{"names": names}
	applySalutationOptions(p, o)
	var result SalutationBulkResult
	return &result, c.request(ctx, "/salutation/bulk", p, &result)
}
func applySalutationOptions(p map[string]any, o SalutationOptions) {
	for key, value := range map[string]string{
		"language": o.Language,
		"country":  o.Country,
		"locale":   o.Locale,
		"ip":       o.IP,
		"gender":   o.Gender,
		"title":    o.Title,
	} {
		if value != "" {
			p[key] = value
		}
	}
	if o.MinProbability > 0 {
		p["min_probability"] = o.MinProbability
	}
}
