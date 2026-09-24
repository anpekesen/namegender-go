package namegender

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// BatchJob is a file job, as POST, GET and the list return it and as a
// webhook carries it in data.object. Nullable fields are pointers.
type BatchJob struct {
	ID     string `json:"id"`
	Status string `json:"status"` // uploaded, queued, processing, completed, failed, cancelled
	Source string `json:"source"` // api or panel
	File   struct {
		Name   string `json:"name"`
		Format string `json:"format"`
	} `json:"file"`
	Columns struct {
		Name    *string `json:"name"`
		Country *string `json:"country"`
	} `json:"columns"`
	Options struct {
		Country             *string `json:"country"`
		AIFallback          bool    `json:"ai_fallback"`
		BestGuess           bool    `json:"best_guess"`
		DeleteAfterDownload bool    `json:"delete_after_download"`
	} `json:"options"`
	Rows struct {
		Total      int  `json:"total"`
		Processed  int  `json:"processed"`
		Identified *int `json:"identified"` // set once completed
	} `json:"rows"`
	Progress int `json:"progress"`
	Credits  struct {
		Reserved *int `json:"reserved"`
		Charged  *int `json:"charged"` // nil until completed; a failed job is not charged
	} `json:"credits"`
	Summary          *BatchSummary    `json:"summary"`
	DataVersion      *string          `json:"data_version"`
	Error            *BatchError      `json:"error"`      // set when Status is failed
	Result           *BatchResult     `json:"result"`     // set when Status is completed
	Inspection       *BatchInspection `json:"inspection"` // only while Status is uploaded
	PollAfterSeconds *int             `json:"poll_after_seconds"`
	CreatedAt        *time.Time       `json:"created_at"`
	StartedAt        *time.Time       `json:"started_at"`
	FinishedAt       *time.Time       `json:"finished_at"`
	ExpiresAt        *time.Time       `json:"expires_at"`
}

type BatchSummary struct {
	Male    int `json:"male"`
	Female  int `json:"female"`
	Unknown int `json:"unknown"`
	FromLLM int `json:"from_llm"`
}

// BatchError explains a failed job. Branch on Code (source_missing,
// no_columns, name_column_missing, empty_file, bad_format, unreadable,
// no_credits, processing_error, stalled), not on Message.
type BatchError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type BatchResult struct {
	URL       string     `json:"url"`
	Format    string     `json:"format"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// BatchInspection is what a job uploaded with NoStart needs to be started:
// the columns, the first rows and the cost.
type BatchInspection struct {
	Columns              []string   `json:"columns"`
	Preview              [][]string `json:"preview"`
	GuessedNameColumn    *string    `json:"guessed_name_column"`
	GuessedCountryColumn *string    `json:"guessed_country_column"`
	CreditsNeeded        int        `json:"credits_needed"`
	CreditsAvailable     int        `json:"credits_available"`
}

type BatchList struct {
	Data    []BatchJob `json:"data"`
	Page    int        `json:"page"`
	PerPage int        `json:"per_page"`
	Total   int        `json:"total"`
	HasMore bool       `json:"has_more"`
}

// BatchSettings starts a job. NameColumn is required: a guessed column that
// turns out to be wrong would spend credits on the wrong data.
type BatchSettings struct {
	NameColumn          string `json:"name_column"`
	CountryColumn       string `json:"country_column,omitempty"`
	Country             string `json:"country,omitempty"`
	AIFallback          bool   `json:"ai_fallback,omitempty"`
	BestGuess           bool   `json:"best_guess,omitempty"`
	DeleteAfterDownload bool   `json:"delete_after_download,omitempty"`
}

// BatchOptions configures CreateBatch. NameColumn is required unless NoStart
// is set; NoStart uploads and inspects only (sent as start=false), and the job
// is started later with StartBatch.
//
// Every attempt of one CreateBatch call sends the same Idempotency-Key, so a
// retry never opens a second job. Set IdempotencyKey to keep that guarantee
// across your own retries. Retries is the number of extra attempts after a
// network error or a 502/503/504: zero means the default of 2, a negative
// value turns retrying off.
type BatchOptions struct {
	NameColumn          string
	CountryColumn       string
	Country             string
	AIFallback          bool
	BestGuess           bool
	DeleteAfterDownload bool
	NoStart             bool

	IdempotencyKey string
	Retries        int
}

// WaitOptions configures WaitBatch. Timeout defaults to one hour; the context
// can end the wait sooner. OnProgress, if set, sees the job after every poll.
type WaitOptions struct {
	Timeout    time.Duration
	OnProgress func(*BatchJob)
}

// sleep waits for d or until ctx ends. A variable so that tests do not wait.
var sleep = func(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// CreateBatch uploads a CSV or XLSX file and, unless opts.NoStart is set,
// starts it. The extension of filename (.csv, .xlsx) tells the API the
// format. The file is read into memory once so that a retry can send it again.
func (c *Client) CreateBatch(ctx context.Context, file io.Reader, filename string, opts BatchOptions) (*BatchJob, error) {
	if filename == "" {
		return nil, errors.New("namegender: filename is required")
	}
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	body, contentType, err := batchForm(content, filename, opts)
	if err != nil {
		return nil, err
	}
	key := opts.IdempotencyKey
	if key == "" {
		if key, err = newIdempotencyKey(); err != nil {
			return nil, err
		}
	}
	header := http.Header{"Idempotency-Key": {key}}
	retries := opts.Retries
	if retries == 0 {
		retries = 2
	}
	for attempt := 0; ; attempt++ {
		raw, err := c.send(ctx, http.MethodPost, "/batches", contentType, body, header)
		if err == nil {
			var job BatchJob
			return &job, json.Unmarshal(raw, &job)
		}
		if attempt >= retries || !retryable(ctx, err) {
			return nil, err
		}
		if err := sleep(ctx, time.Second<<attempt); err != nil {
			return nil, err
		}
	}
}

// CreateBatchFile is CreateBatch for a file on disk; its base name is sent as
// the file name.
func (c *Client) CreateBatchFile(ctx context.Context, path string, opts BatchOptions) (*BatchJob, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return c.CreateBatch(ctx, f, filepath.Base(path), opts)
}

// StartBatch starts a job uploaded with NoStart.
func (c *Client) StartBatch(ctx context.Context, id string, settings BatchSettings) (*BatchJob, error) {
	if settings.NameColumn == "" {
		return nil, errors.New("namegender: NameColumn is required")
	}
	var job BatchJob
	return &job, c.request(ctx, "/batches/"+url.PathEscape(id)+"/start", settings, &job)
}

func (c *Client) GetBatch(ctx context.Context, id string) (*BatchJob, error) {
	raw, err := c.send(ctx, http.MethodGet, "/batches/"+url.PathEscape(id), "", nil, nil)
	if err != nil {
		return nil, err
	}
	var job BatchJob
	return &job, json.Unmarshal(raw, &job)
}

// ListBatches returns jobs newest first, including those started from the
// dashboard. Zero limit or page leaves the server default.
func (c *Client) ListBatches(ctx context.Context, limit, page int) (*BatchList, error) {
	query := url.Values{}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}
	path := "/batches"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	raw, err := c.send(ctx, http.MethodGet, path, "", nil, nil)
	if err != nil {
		return nil, err
	}
	var list BatchList
	return &list, json.Unmarshal(raw, &list)
}

// CancelBatch cancels a job that has not started (its credit is returned), or
// deletes a finished one.
func (c *Client) CancelBatch(ctx context.Context, id string) error {
	_, err := c.send(ctx, http.MethodDelete, "/batches/"+url.PathEscape(id), "", nil, nil)
	return err
}

// DownloadBatch returns the result file, in the format that was uploaded, as
// bytes held in memory. A CSV result starts with a UTF-8 byte order mark.
func (c *Client) DownloadBatch(ctx context.Context, id string) ([]byte, error) {
	return c.send(ctx, http.MethodGet, "/batches/"+url.PathEscape(id)+"/result", "", nil, nil)
}

// WaitBatch polls until the job is completed, failed or cancelled (or is
// uploaded and waiting for StartBatch) and returns it. A failed job is
// returned without an error: check Status and Error.Code.
func (c *Client) WaitBatch(ctx context.Context, id string, opts WaitOptions) (*BatchJob, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = time.Hour
	}
	deadline := time.Now().Add(timeout)
	for {
		job, err := c.GetBatch(ctx, id)
		if err != nil {
			return nil, err
		}
		if opts.OnProgress != nil {
			opts.OnProgress(job)
		}
		switch job.Status {
		case "completed", "failed", "cancelled", "uploaded":
			return job, nil
		}
		pause := 5 * time.Second
		if job.PollAfterSeconds != nil && *job.PollAfterSeconds > 0 {
			pause = time.Duration(*job.PollAfterSeconds) * time.Second
		}
		if time.Now().Add(pause).After(deadline) {
			return job, fmt.Errorf("namegender: timed out waiting for %s", id)
		}
		if err := sleep(ctx, pause); err != nil {
			return job, err
		}
	}
}

// retryable reports whether an upload is worth sending again: the request may
// never have reached the application. Everything else (402, 422, 429
// too_many_batches) would fail the same way again.
func retryable(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status == 502 || apiErr.Status == 503 || apiErr.Status == 504
	}
	return true
}

func batchForm(content []byte, filename string, o BatchOptions) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fields := [][2]string{{"start", strconv.FormatBool(!o.NoStart)}}
	for _, f := range [][2]string{
		{"name_column", o.NameColumn},
		{"country_column", o.CountryColumn},
		{"country", o.Country},
	} {
		if f[1] != "" {
			fields = append(fields, f)
		}
	}
	for _, f := range []struct {
		name string
		on   bool
	}{{"ai_fallback", o.AIFallback}, {"best_guess", o.BestGuess}, {"delete_after_download", o.DeleteAfterDownload}} {
		if f.on {
			fields = append(fields, [2]string{f.name, "true"})
		}
	}
	for _, f := range fields {
		if err := w.WriteField(f[0], f[1]); err != nil {
			return nil, "", err
		}
	}
	// CreateFormFile would label the part by extension; the API reads the
	// extension of the file name itself, so a plain octet-stream is enough.
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, quoteFilename(filename)))
	h.Set("Content-Type", "application/octet-stream")
	part, err := w.CreatePart(h)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(content); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}

// quoteFilename keeps a quote or line break in a file name from ending the
// header early.
func quoteFilename(name string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, "%22", "\r", "", "\n", "").Replace(name)
}

// newIdempotencyKey returns a random (version 4) UUID.
func newIdempotencyKey() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
