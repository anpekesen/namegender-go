package namegender

import "context"

// NameCheckOptions configures NameCheck and NameCheckBulk. Empty fields are
// not sent.
//
// FirstName and LastName replace the name when the two are stored separately;
// they are not parsed. NameCheckBulk ignores them.
//
// Country, Locale and IP are the same country hints as in Options.
type NameCheckOptions struct {
	FirstName string
	LastName  string
	Country   string
	Locale    string
	IP        string
}

// NameCheckItem says whether one name looks like a real person's name. It
// never calls a name fake: use it to flag records for review, not to reject
// people automatically. Nullable fields are pointers.
type NameCheckItem struct {
	Query      string            `json:"query"`
	Assessment string            `json:"assessment"` // plausible, suspicious or implausible
	Score      int               `json:"score"`      // 0-100
	Signals    []NameCheckSignal `json:"signals"`
	FirstName  *string           `json:"first_name"`
	LastName   *string           `json:"last_name"`
	NameType   string            `json:"name_type"` // personal, organization or role
	Evidence   NameCheckEvidence `json:"evidence"`
}

// NameCheckSignal is one reason behind an assessment, such as
// keyboard_pattern, placeholder or first_name_attested.
type NameCheckSignal struct {
	Code     string  `json:"code"`
	Severity string  `json:"severity"` // high, medium, low, info or positive
	Part     *string `json:"part"`     // full, first_name, last_name or nil
	Value    *string `json:"value"`
}

// NameCheckEvidence is what the database knows about the first name.
// Surnames are judged by their shape only.
type NameCheckEvidence struct {
	FirstNameStatus         *string `json:"first_name_status"` // counted, attested, not_found or nil
	FirstNameCountedRecords int     `json:"first_name_counted_records"`
}

// NameCheckResult is the response of NameCheck.
type NameCheckResult struct {
	NameCheckItem
	CountrySource    *string `json:"country_source"` // country, locale, ip or nil
	CreditsCharged   int     `json:"credits_charged"`
	CreditsRemaining int     `json:"credits_remaining"`
	DataVersion      *string `json:"data_version"`
	RequestID        *string `json:"request_id"`
}

// NameCheckBulkResult is the response of NameCheckBulk. Results are in input
// order.
type NameCheckBulkResult struct {
	Results          []NameCheckItem  `json:"results"`
	Summary          NameCheckSummary `json:"summary"`
	CountrySource    *string          `json:"country_source"` // country, locale, ip or nil
	TookMS           int              `json:"took_ms"`
	CreditsCharged   int              `json:"credits_charged"`
	CreditsRemaining int              `json:"credits_remaining"`
	DataVersion      *string          `json:"data_version"`
	RequestID        *string          `json:"request_id"`
}

// NameCheckSummary counts the assessments in a NameCheckBulkResult.
type NameCheckSummary struct {
	Total       int `json:"total"`
	Plausible   int `json:"plausible"`
	Suspicious  int `json:"suspicious"`
	Implausible int `json:"implausible"`
}

// NameCheck says whether a name typed into a form looks like a real person's
// name, with the reasons. name may be empty when FirstName or LastName is set.
// One credit.
func (c *Client) NameCheck(ctx context.Context, name string, o NameCheckOptions) (*NameCheckResult, error) {
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
	applyNameCheckOptions(p, o)
	var result NameCheckResult
	return &result, c.request(ctx, "/name-check", p, &result)
}

// NameCheckBulk checks up to 100 names, one credit each.
func (c *Client) NameCheckBulk(ctx context.Context, names []string, o NameCheckOptions) (*NameCheckBulkResult, error) {
	p := map[string]any{"names": names}
	applyNameCheckOptions(p, o)
	var result NameCheckBulkResult
	return &result, c.request(ctx, "/name-check/bulk", p, &result)
}
func applyNameCheckOptions(p map[string]any, o NameCheckOptions) {
	for key, value := range map[string]string{
		"country": o.Country,
		"locale":  o.Locale,
		"ip":      o.IP,
	} {
		if value != "" {
			p[key] = value
		}
	}
}
