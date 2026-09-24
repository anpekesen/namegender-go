# NameGender Go

```sh
go get github.com/anpekesen/namegender-go
```

```go
client := namegender.New(os.Getenv("NAMEGENDER_API_KEY"))
result, err := client.Name(ctx, "Ayşe", namegender.Options{Country: "TR"})
```

## Options and response

`Options` carries `Country`, `AIFallback` (sent as `ai_fallback`) and
`BestGuess` (sent as `best_guess`). A `Result` has `Query`, `Name`, `FirstName`, `MiddleName`, `LastName`, `NameType`, `Gender`,
`Country`, `Probability`, `SampleSize`, `TookMS`, `Source`, `Confidence` and
`MatchedAs`, plus `CreditsCharged`, `CreditsRemaining`, `DataVersion` and
`RequestID`. Success is the HTTP status: a non-2xx response is returned as an
`*APIError` holding the status and the raw error body
(`{"error", "message", "request_id", "docs"}`).

## Country distribution

Which countries a name is recorded in. This is not a country-of-origin or
ethnicity inference: `Registrations` is counted volume, comparable only among
the countries that publish counted birth statistics, and `AttestedIn` is
presence with no weight attached. Show `Basis.Note` next to any percentage.

```go
dist, err := client.Countries(ctx, "Mehmet", namegender.CountriesOptions{Limit: 10})
for _, r := range dist.Registrations {
	fmt.Printf("%s %.2f%%\n", r.Country, r.Share)
}
fmt.Println(strings.Join(dist.AttestedIn, ", "))
```

## File jobs

Upload a CSV or XLSX file (up to 100 MB and 1,000,000 rows) and get it back
with gender columns added. One credit per row, charged only if the job
completes.

```go
job, err := client.CreateBatchFile(ctx, "customers.csv", namegender.BatchOptions{
	NameColumn:    "first_name", // required to start
	CountryColumn: "country",    // optional: a country code per row
})
if err != nil {
	return err
}

done, err := client.WaitBatch(ctx, job.ID, namegender.WaitOptions{
	OnProgress: func(j *namegender.BatchJob) { log.Println(j.Progress) },
})
if err != nil {
	return err
}
if done.Status == "failed" {
	return errors.New(done.Error.Code)
}

file, err := client.DownloadBatch(ctx, done.ID)
if err != nil {
	return err
}
err = os.WriteFile("customers-gender.csv", file, 0o644)
```

`CreateBatch` takes any `io.Reader` and a file name instead of a path; the
extension (`.csv`, `.xlsx`) tells the API the format. The file is read into
memory once, so that a retry can send it again, and `DownloadBatch` returns
the result as a `[]byte`.

`NameColumn` is required to start: a guessed column that turns out to be
wrong would spend credits on the wrong data. To see the columns and the cost
first, upload with `NoStart: true`, read `job.Inspection`, then call
`client.StartBatch(ctx, job.ID, namegender.BatchSettings{NameColumn: ...})`.

`CreateBatch` sends an `Idempotency-Key` and retries network errors and
502/503/504 with the same key, so a retry never opens a second job. Set your
own `IdempotencyKey` to keep that guarantee across your own retries.
`Retries` is the number of extra attempts (default 2, negative for none).

`WaitBatch` returns a failed job without an error; branch on `job.Error.Code`.
It polls as often as the job's `PollAfterSeconds` says, gives up after
`WaitOptions.Timeout` (default one hour) and stops when the context ends.
`CancelBatch` returns the credit of a job that has not started, and deletes a
finished one. `ListBatches(ctx, limit, page)` includes jobs started from the
dashboard. Up to three jobs can be queued or running at once; a fourth is
refused with `429 too_many_batches`.

The result appends `gender`, `probability`, `sample_size`, `country`, `source`,
`matched_as`, `first_name`, `middle_name`, `last_name` and `name_type` to every
row. A CSV result starts with a UTF-8 byte order mark so that Excel reads it
correctly; strip `"\xef\xbb\xbf"` before handing it to `encoding/csv`.

## Webhooks

Add an endpoint under Webhooks in the dashboard, and NameGender sends a signed
`POST` to it when a file job completes or fails, and when credits are about to
run out (`credits.low`) or have run out (`credits.depleted`, checked hourly).
`VerifyWebhook` checks the signature and the timestamp, and returns the event.

```go
http.HandleFunc("/namegender", func(w http.ResponseWriter, r *http.Request) {
	// The raw bytes, read before any JSON decoding: the signature covers
	// the exact body sent.
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	event, err := namegender.VerifyWebhook(payload, r.Header.Get("NameGender-Signature"), os.Getenv("NAMEGENDER_WEBHOOK_SECRET"))
	if errors.Is(err, namegender.ErrWebhookSignature) {
		http.Error(w, "", http.StatusBadRequest)
		return
	} else if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	// Answer within 10 seconds: hand slow work to a queue or a goroutine.
	go handle(event)
	w.WriteHeader(http.StatusNoContent)
})
```

Use `event.ID` (also the `NameGender-Event-Id` header) to ignore a delivery you
have already handled. A retry carries the same id, and order is not guaranteed.
Anything other than a 2xx within 10 seconds is retried, up to 8 attempts over
about 45 hours.

```go
func handle(event *namegender.WebhookEvent) {
	switch event.Type {
	case "batch.completed", "batch.failed":
		job, err := event.Batch() // the job, as GetBatch returns it
		// ...
	case "credits.low", "credits.depleted":
		alert, err := event.CreditsAlert() // alert.CreditsRemaining, alert.RunwayDays
		// ...
	}
}
```

The timestamp may be at most 5 minutes from your clock;
`namegender.WithTolerance` changes that. During a secret rotation the header
carries two signatures, and either one matching is enough.
