package namegender

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// noSleep makes backoff and polling instant for the duration of a test and
// records every pause asked for.
func noSleep(t *testing.T) *[]time.Duration {
	var pauses []time.Duration
	original := sleep
	sleep = func(ctx context.Context, d time.Duration) error { pauses = append(pauses, d); return ctx.Err() }
	t.Cleanup(func() { sleep = original })
	return &pauses
}

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c := New("secret")
	c.BaseURL = server.URL
	return c
}

func TestCreateBatchMultipart(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/batches" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("request=%s %s", r.Method, r.URL.Path)
		}
		if len(r.Header.Get("Idempotency-Key")) != 36 {
			t.Errorf("idempotency key=%q", r.Header.Get("Idempotency-Key"))
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		want := map[string]string{"name_column": "first_name", "country_column": "country", "best_guess": "true", "start": "true"}
		for key, value := range want {
			if r.FormValue(key) != value {
				t.Errorf("%s=%q", key, r.FormValue(key))
			}
		}
		for _, key := range []string{"country", "ai_fallback", "delete_after_download"} {
			if _, ok := r.MultipartForm.Value[key]; ok {
				t.Errorf("%s sent though unset", key)
			}
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		content, _ := io.ReadAll(file)
		if header.Filename != "customers.csv" || string(content) != "first_name,country\nAyşe,TR\n" {
			t.Errorf("file=%s %q", header.Filename, content)
		}
		w.WriteHeader(201)
		io.WriteString(w, `{"id":"B-1","status":"queued","rows":{"total":1,"processed":0,"identified":null},"poll_after_seconds":2,"created_at":"2026-09-24T10:00:00Z"}`)
	})
	job, err := c.CreateBatch(context.Background(), strings.NewReader("first_name,country\nAyşe,TR\n"), "customers.csv", BatchOptions{NameColumn: "first_name", CountryColumn: "country", BestGuess: true})
	if err != nil || job.ID != "B-1" || job.Status != "queued" || job.Rows.Total != 1 || job.Rows.Identified != nil || *job.PollAfterSeconds != 2 || job.CreatedAt.Year() != 2026 {
		t.Fatalf("job=%+v err=%v", job, err)
	}
}

func TestCreateBatchNoStart(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("start") != "false" || r.Header.Get("Idempotency-Key") != "mine" {
			t.Errorf("start=%q key=%q", r.FormValue("start"), r.Header.Get("Idempotency-Key"))
		}
		io.WriteString(w, `{"id":"B-2","status":"uploaded","inspection":{"columns":["id","first_name"],"preview":[["1","Ayşe"]],"guessed_name_column":"first_name","guessed_country_column":null,"credits_needed":1,"credits_available":50}}`)
	})
	job, err := c.CreateBatch(context.Background(), strings.NewReader("x"), "a.csv", BatchOptions{NoStart: true, IdempotencyKey: "mine"})
	if err != nil || job.Inspection == nil || len(job.Inspection.Columns) != 2 || job.Inspection.GuessedCountryColumn != nil || job.Inspection.Preview[0][1] != "Ayşe" {
		t.Fatalf("job=%+v err=%v", job, err)
	}
}

func TestCreateBatchRetriesWithSameKey(t *testing.T) {
	pauses := noSleep(t)
	var keys []string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		file, _, _ := r.FormFile("file")
		if content, _ := io.ReadAll(file); string(content) != "data" {
			t.Errorf("attempt %d body=%q", len(keys), content)
		}
		if len(keys) < 3 {
			w.WriteHeader(503)
			return
		}
		io.WriteString(w, `{"id":"B-3","status":"queued"}`)
	})
	job, err := c.CreateBatch(context.Background(), strings.NewReader("data"), "a.csv", BatchOptions{NameColumn: "n"})
	if err != nil || job.ID != "B-3" {
		t.Fatalf("job=%+v err=%v", job, err)
	}
	if len(keys) != 3 || keys[0] == "" || keys[1] != keys[0] || keys[2] != keys[0] {
		t.Fatalf("keys=%v", keys)
	}
	if len(*pauses) != 2 || (*pauses)[0] != time.Second || (*pauses)[1] != 2*time.Second {
		t.Fatalf("pauses=%v", *pauses)
	}
}

func TestCreateBatchGivesUpAfterRetries(t *testing.T) {
	noSleep(t)
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(502) })
	_, err := c.CreateBatch(context.Background(), strings.NewReader("x"), "a.csv", BatchOptions{NameColumn: "n", Retries: 1})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 502 || calls != 2 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestCreateBatchDoesNotRetry4xx(t *testing.T) {
	noSleep(t)
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(402)
		io.WriteString(w, `{"error":"no_credits"}`)
	})
	_, err := c.CreateBatch(context.Background(), strings.NewReader("x"), "a.csv", BatchOptions{NameColumn: "n"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 402 || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestStartBatch(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Method != http.MethodPost || r.URL.EscapedPath() != "/batches/B%2F1/start" || string(body) != `{"name_column":"first_name","country_column":"country"}` {
			t.Errorf("%s %s %s", r.Method, r.URL.EscapedPath(), body)
		}
		io.WriteString(w, `{"id":"B/1","status":"queued"}`)
	})
	job, err := c.StartBatch(context.Background(), "B/1", BatchSettings{NameColumn: "first_name", CountryColumn: "country"})
	if err != nil || job.Status != "queued" {
		t.Fatalf("job=%+v err=%v", job, err)
	}
	if _, err := c.StartBatch(context.Background(), "B-1", BatchSettings{}); err == nil {
		t.Fatal("empty NameColumn accepted")
	}
}

func TestWaitBatchPollsUntilFinished(t *testing.T) {
	pauses := noSleep(t)
	polls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		polls++
		if r.Method != http.MethodGet || r.URL.Path != "/batches/B-1" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		switch polls {
		case 1:
			io.WriteString(w, `{"id":"B-1","status":"queued","progress":0,"poll_after_seconds":2}`)
		case 2:
			io.WriteString(w, `{"id":"B-1","status":"processing","progress":50,"poll_after_seconds":null}`)
		default:
			io.WriteString(w, `{"id":"B-1","status":"failed","progress":50,"error":{"code":"no_credits","message":"m"}}`)
		}
	})
	var progress []int
	job, err := c.WaitBatch(context.Background(), "B-1", WaitOptions{OnProgress: func(j *BatchJob) { progress = append(progress, j.Progress) }})
	if err != nil || job.Status != "failed" || job.Error == nil || job.Error.Code != "no_credits" {
		t.Fatalf("job=%+v err=%v", job, err)
	}
	if polls != 3 || len(progress) != 3 || progress[1] != 50 {
		t.Fatalf("polls=%d progress=%v", polls, progress)
	}
	if len(*pauses) != 2 || (*pauses)[0] != 2*time.Second || (*pauses)[1] != 5*time.Second {
		t.Fatalf("pauses=%v", *pauses)
	}
}

func TestWaitBatchStopsWithContext(t *testing.T) {
	noSleep(t)
	ctx, cancel := context.WithCancel(context.Background())
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		cancel()
		io.WriteString(w, `{"id":"B-1","status":"processing","poll_after_seconds":2}`)
	})
	if _, err := c.WaitBatch(ctx, "B-1", WaitOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestCancelListDownload(t *testing.T) {
	var seen []string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.RequestURI())
		switch {
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/result"):
			w.Header().Set("Content-Type", "text/csv")
			io.WriteString(w, "\xef\xbb\xbfid,gender\n1,female\n")
		default:
			io.WriteString(w, `{"data":[{"id":"B-1","status":"completed","summary":{"male":1,"female":2,"unknown":0,"from_llm":0}}],"page":2,"per_page":10,"total":11,"has_more":false}`)
		}
	})
	ctx := context.Background()
	if err := c.CancelBatch(ctx, "B 1"); err != nil {
		t.Fatal(err)
	}
	list, err := c.ListBatches(ctx, 10, 2)
	if err != nil || len(list.Data) != 1 || list.Data[0].Summary.Female != 2 || list.Total != 11 || list.PerPage != 10 {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	file, err := c.DownloadBatch(ctx, "B-1")
	if err != nil || !strings.HasPrefix(string(file), "\xef\xbb\xbfid,gender") {
		t.Fatalf("file=%q err=%v", file, err)
	}
	if _, err := c.ListBatches(ctx, 0, 0); err != nil {
		t.Fatal(err)
	}
	want := []string{"DELETE /batches/B%201", "GET /batches?limit=10&page=2", "GET /batches/B-1/result", "GET /batches"}
	if strings.Join(seen, "|") != strings.Join(want, "|") {
		t.Fatalf("seen=%v", seen)
	}
}
