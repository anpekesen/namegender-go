package namegender

import "context"

// AgeOptions configures Age and AgeBulk. Empty fields are not sent.
//
// Gender ("male" or "female") narrows the estimate to one gender's records.
// Country, Locale and IP are the same country hints as in Options.
type AgeOptions struct {
	Gender  string
	Country string
	Locale  string
	IP      string
}

// AgeRange is a range of ages, both ends inclusive.
type AgeRange struct {
	Low  int `json:"low"`
	High int `json:"high"`
}

// AgeItem estimates the age of the people who carry a first name, from
// birth records. It describes a group, not a person: never use it for
// decisions about an individual. Nullable fields are pointers.
//
// When Age is nil, Reason says why: not_found, insufficient_data or
// country_not_covered (no credit charged). That is a normal answer, not an
// error.
type AgeItem struct {
	Name          string    `json:"name"`
	FirstName     *string   `json:"first_name"`
	Gender        *string   `json:"gender"`       // male, female or nil
	Age           *int      `json:"age"`          // median age
	AgeRange      *AgeRange `json:"age_range"`    // middle half
	AgeRange80    *AgeRange `json:"age_range_80"` // middle 80 percent
	BirthYear     *int      `json:"birth_year"`
	SampleSize    int       `json:"sample_size"`
	Births        int       `json:"births"`
	Country       string    `json:"country"`
	CountrySource string    `json:"country_source"` // country, locale, ip or default
	Source        *string   `json:"source"`
	Series        *string   `json:"series"`
	ReferenceYear int       `json:"reference_year"`
	Reason        *string   `json:"reason"` // not_found, insufficient_data, country_not_covered or nil
}

// AgeResult is the response of Age.
type AgeResult struct {
	AgeItem
	CreditsCharged   int     `json:"credits_charged"`
	CreditsRemaining int     `json:"credits_remaining"`
	RequestID        *string `json:"request_id"`
}

// AgeBulkResult is the response of AgeBulk. Results are in input order.
type AgeBulkResult struct {
	Results          []AgeItem `json:"results"`
	CountrySource    string    `json:"country_source"` // country, locale, ip or default
	CreditsCharged   int       `json:"credits_charged"`
	CreditsRemaining int       `json:"credits_remaining"`
	RequestID        *string   `json:"request_id"`
}

// Age estimates the age of the people who carry a first name: the median,
// the middle half and the middle 80 percent. It covers the US, France and
// Norway. One credit; none when the country is not covered.
func (c *Client) Age(ctx context.Context, name string, o AgeOptions) (*AgeResult, error) {
	p := map[string]any{"name": name}
	applyAgeOptions(p, o)
	var result AgeResult
	return &result, c.request(ctx, "/age", p, &result)
}

// AgeBulk estimates ages for up to 100 names, one credit each. The options
// apply to every name.
func (c *Client) AgeBulk(ctx context.Context, names []string, o AgeOptions) (*AgeBulkResult, error) {
	p := map[string]any{"names": names}
	applyAgeOptions(p, o)
	var result AgeBulkResult
	return &result, c.request(ctx, "/age/bulk", p, &result)
}
func applyAgeOptions(p map[string]any, o AgeOptions) {
	for key, value := range map[string]string{
		"gender":  o.Gender,
		"country": o.Country,
		"locale":  o.Locale,
		"ip":      o.IP,
	} {
		if value != "" {
			p[key] = value
		}
	}
}
