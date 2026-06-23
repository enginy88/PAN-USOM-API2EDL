package sgb

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/enginy88/PAN-SGB-API2EDL/config"
	"github.com/enginy88/PAN-SGB-API2EDL/logger"

	"github.com/go-resty/resty/v2"
)

var (
	ErrEmptyAPIPath   = errors.New("empty api path")
	ErrFailedResponse = errors.New("response with non 2xx status code")
	ErrTypeAssert     = errors.New("type assertion failure")
	ErrPageValidation = errors.New("page validation failure")
	ErrTotalMismatch  = errors.New("total record count mismatch")
)

type APIRequest struct {
	ctx        context.Context
	cancelFunc context.CancelFunc
	apiPath    string
	client     *resty.Client
}

// This is to implement resty.Logger interface
type restyLogger struct{}

func (l restyLogger) Debugf(_ string, _ ...any) {}
func (l restyLogger) Warnf(format string, v ...any) {
	logger.LogInfo.Printf("RESTY: "+format, v...)
}
func (l restyLogger) Errorf(format string, v ...any) {
	logger.LogWarn.Printf("RESTY: "+format, v...)
}

// headerToString converts http.Header to string
func headerToString(headers http.Header) string {
	var sb strings.Builder
	if err := headers.Write(&sb); err != nil {
		logger.LogWarn.Println("FETCH: Failed to write headers! (" + err.Error() + ")")
		return ""
	}
	return sb.String()
}

func newAPIRequest(ctx context.Context, apiPath string) (*APIRequest, error) {

	if apiPath == "" {
		return nil, ErrEmptyAPIPath
	}

	// Use background context if no context provided
	if ctx == nil {
		ctx = context.Background()
	}

	// Create a cancelable context
	ctx, cancelFunc := context.WithTimeout(ctx, time.Duration(config.AppEnv.Request.TotalTimeout)*time.Second)

	es := &APIRequest{
		ctx:        ctx,
		cancelFunc: cancelFunc,
		apiPath:    apiPath,
		client:     resty.New(),
	}

	// Set default values
	es.client.EnableTrace()
	es.client.SetLogger(restyLogger{})
	es.client.SetJSONEscapeHTML(true)
	es.client.SetCloseConnection(true)
	es.client.SetContentLength(true)
	es.client.SetResponseBodyLimit(config.AppEnv.Request.ResponseBodyLimit)
	es.client.SetRetryCount(config.AppEnv.Request.AddRetryCount)
	es.client.SetRetryWaitTime(time.Duration(config.AppEnv.Request.RetryWaitTime) * time.Millisecond)
	es.client.SetRetryMaxWaitTime(time.Duration(config.AppEnv.Request.RetryMaxWaitTime) * time.Millisecond)
	es.client.SetTimeout(time.Duration(config.AppEnv.Request.RequestTimeout) * time.Second)
	es.client.SetHeaders(map[string]string{
		"Accept":       "application/json; charset=utf-8",
		"Content-Type": "application/json; charset=utf-8",
		"User-Agent":   config.AppEnv.Request.UserAgent,
	})

	if config.AppEnv.Request.AllowRedirect {
		es.client.SetRedirectPolicy(resty.FlexibleRedirectPolicy(config.AppEnv.Request.MaxRedirect))
	} else {
		es.client.SetRedirectPolicy(resty.NoRedirectPolicy())
	}

	return es, nil
}

func (req *APIRequest) fetchPage(ctx context.Context, page int, config *Config) (*Response, error) {

	params := map[string]string{
		"page": strconv.Itoa(page),
	}

	// Add optional parameters only if config is provided and values are set
	if config != nil {
		if config.AddressType != "" {
			params["type"] = config.AddressType
		}
		if config.CriticalityLevel > 0 {
			params["criticality_level"] = strconv.Itoa(config.CriticalityLevel)
		}
		if config.DateGTE != "" {
			params["date_gte"] = config.DateGTE
		}
		if config.DateLTE != "" {
			params["date_lte"] = config.DateLTE
		}
		if config.Source != "" {
			params["source"] = config.Source
		}
		if config.Desc != "" {
			params["desc"] = config.Desc
		}
		if config.ConnectionType != "" {
			params["connectiontype"] = config.ConnectionType
		}
		if config.PerPage > 0 {
			params["per-page"] = strconv.Itoa(config.PerPage)
		}
	}

	r := req.client.R().SetContext(ctx)
	r = r.SetQueryParams(params).SetResult(&Response{})

	resp, err := r.Execute(resty.MethodGet, req.apiPath)

	if err != nil {
		logger.LogErr.Println("FETCH: Failed to execute API call! (" + err.Error() + ")")
		return nil, err
	}

	logger.LogInfo.Println("FETCH: API call executed, trace info: " + logger.Explain(resp.Request.TraceInfo()) + ")")

	if !resp.IsSuccess() {
		logger.LogErr.Println("FETCH: API call failed with status: " + resp.Status() + " (PROTO: " + resp.Proto() + " BODY: " + resp.String() + ")")
		return nil, ErrFailedResponse
	}

	logger.LogInfo.Println("FETCH: API call succeed with status: " + resp.Status() + " (PROTO: " + resp.Proto() + ")")
	logger.LogDebug.Println("FETCH: API call response header: " + headerToString(resp.Header()))

	result, ok := resp.Result().(*Response)
	if !ok {
		logger.LogErr.Println("FETCH: Type assertion failure! (" + logger.Typeof(result) + " -> *Response)")
		return nil, ErrTypeAssert
	}
	return result, nil

}

func (req *APIRequest) cancelRequest() {
	if req.cancelFunc != nil {
		req.cancelFunc()
	}
}

// validatePage verifies that a fetched page is internally consistent. The
// reported count must equal the number of received records, and every page is
// expected to be full (per-page) except the last page, which holds the
// remainder of the total record count.
func validatePage(resp *Response, page, perPage, totalCount, pageCount int) error {

	// The reported count must match the actual number of received records.
	if resp.Count != len(resp.Models) {
		logger.LogWarn.Println("FETCH: Inconsistency on page " + strconv.Itoa(page) + "! (reported count " + strconv.Itoa(resp.Count) + " but received " + strconv.Itoa(len(resp.Models)) + " records)")
		return ErrPageValidation
	}

	// Without a known per-page size the expected count cannot be derived.
	if perPage <= 0 {
		return nil
	}

	// Every page should carry per-page records, except the last page which
	// holds the remainder (this also covers the case where the total count is
	// smaller than the per-page size, leaving a single, last page).
	expected := perPage
	if page == pageCount {
		expected = totalCount - (pageCount-1)*perPage
	}

	if resp.Count != expected {
		logger.LogWarn.Println("FETCH: Inconsistency on page " + strconv.Itoa(page) + "! (expected " + strconv.Itoa(expected) + " records but got " + strconv.Itoa(resp.Count) + ")")
		return ErrPageValidation
	}

	return nil
}

// fetchAndValidate fetches a single page and validates its content, refetching
// up to AddRefetchCount additional times when an inconsistency is found.
func (req *APIRequest) fetchAndValidate(ctx context.Context, page int, cfg *Config, totalCount, pageCount int) (*Response, error) {

	maxAttempts := config.AppEnv.Request.AddRefetchCount
	if maxAttempts < 0 {
		maxAttempts = 0
	}

	var lastErr error

	for attempt := 0; attempt <= maxAttempts; attempt++ {
		// Stop refetching as soon as the context is cancelled (e.g. another
		// worker failed) instead of burning through the remaining attempts.
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if attempt > 0 {
			logger.LogWarn.Println("FETCH: Refetching page " + strconv.Itoa(page) + " (attempt " + strconv.Itoa(attempt+1) + "/" + strconv.Itoa(maxAttempts+1) + ")...")
		}

		resp, err := req.fetchPage(ctx, page, cfg)
		if err != nil {
			lastErr = err
			continue
		}

		if vErr := validatePage(resp, page, cfg.PerPage, totalCount, pageCount); vErr != nil {
			lastErr = vErr
			continue
		}

		return resp, nil
	}

	logger.LogErr.Println("FETCH: Giving up on page " + strconv.Itoa(page) + " after " + strconv.Itoa(maxAttempts+1) + " attempts! (" + lastErr.Error() + ")")
	return nil, lastErr
}

// fetchFirstPage fetches and validates page 1, deriving the total/page counts
// from the response itself.
func (req *APIRequest) fetchFirstPage(ctx context.Context, cfg *Config) (*Response, error) {

	maxAttempts := config.AppEnv.Request.AddRefetchCount
	if maxAttempts < 0 {
		maxAttempts = 0
	}

	var lastErr error

	for attempt := 0; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if attempt > 0 {
			logger.LogWarn.Println("FETCH: Refetching page 1 (attempt " + strconv.Itoa(attempt+1) + "/" + strconv.Itoa(maxAttempts+1) + ")...")
		}

		resp, err := req.fetchPage(ctx, 1, cfg)
		if err != nil {
			lastErr = err
			continue
		}

		if vErr := validatePage(resp, 1, cfg.PerPage, resp.TotalCount, resp.PageCount); vErr != nil {
			lastErr = vErr
			continue
		}

		return resp, nil
	}

	return nil, lastErr
}

// fetchPagesSequentially fetches pages 2..pageCount one by one, storing each
// page's records into pageModels at its page index.
func (req *APIRequest) fetchPagesSequentially(ctx context.Context, cfg *Config, totalCount, pageCount int, pageModels [][]Model) error {

	for page := 2; page <= pageCount; page++ {
		resp, err := req.fetchAndValidate(ctx, page, cfg, totalCount, pageCount)
		if err != nil {
			return err
		}

		pageModels[page] = resp.Models
		logger.LogInfo.Println("FETCH: Fetched page " + strconv.Itoa(page) + "/" + strconv.Itoa(pageCount) + ", got " + strconv.Itoa(len(resp.Models)) + " records.")
	}

	return nil
}

// fetchPagesConcurrently fetches pages 2..pageCount using a pool of workers,
// storing each page's records into pageModels at its page index. Each worker
// writes a distinct index, so no locking is needed around pageModels.
func (req *APIRequest) fetchPagesConcurrently(ctx context.Context, cfg *Config, totalCount, pageCount int, pageModels [][]Model) error {

	remainingPages := pageCount - 1

	numWorkers := config.AppEnv.Global.NumOfWorker
	if numWorkers < 1 {
		numWorkers = 1
	}
	if numWorkers > remainingPages {
		numWorkers = remainingPages
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var fetchErr error

	jobs := make(chan int, remainingPages)

	// Derive a cancelable context so workers stop dispatching new fetches and
	// in-flight requests are aborted as soon as one of them fails.
	fetchCtx, cancelFetch := context.WithCancel(ctx)
	defer cancelFetch()

	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for page := range jobs {
				select {
				case <-fetchCtx.Done():
					return
				default:
				}

				resp, err := req.fetchAndValidate(fetchCtx, page, cfg, totalCount, pageCount)
				if err != nil {
					mu.Lock()
					if fetchErr == nil {
						fetchErr = err
					}
					mu.Unlock()
					cancelFetch()
					return
				}

				pageModels[page] = resp.Models
				logger.LogInfo.Println("FETCH: Fetched page " + strconv.Itoa(page) + "/" + strconv.Itoa(pageCount) + ", got " + strconv.Itoa(len(resp.Models)) + " records.")
			}
		}()
	}

	// The jobs channel is buffered to hold every remaining page, so these sends
	// never block even if a worker has already exited due to an error.
	for page := 2; page <= pageCount; page++ {
		jobs <- page
	}
	close(jobs)

	wg.Wait()

	return fetchErr
}

func FetchAllPages(ctx context.Context) error {

	req, err := newAPIRequest(ctx, config.AppEnv.Global.APIPath)
	if err != nil {
		logger.LogErr.Println("FETCH: Failed to API Fetcher! (" + err.Error() + ")")
		return err
	}

	defer req.cancelRequest()

	cfg := &Config{
		PerPage: config.AppEnv.Request.PerPage,
	}

	// Fetch the first page to learn the total record and page counts.
	firstPage, err := req.fetchFirstPage(req.ctx, cfg)
	if err != nil {
		logger.LogErr.Println("FETCH: Error fetching first page! (" + err.Error() + ")")
		return err
	}

	totalCount := firstPage.TotalCount
	pageCount := firstPage.PageCount

	// Treat a missing/zero page count as a single page, since the first page has
	// already been fetched. This also guards the pageModels indexing below.
	if pageCount < 1 {
		pageCount = 1
	}

	logger.LogInfo.Println("FETCH: Fetched page 1/" + strconv.Itoa(pageCount) + ", got " + strconv.Itoa(len(firstPage.Models)) + " records.")

	// Collect each page's records by page index to preserve overall ordering
	// regardless of the order pages complete in.
	pageModels := make([][]Model, pageCount+1)
	pageModels[1] = firstPage.Models

	// Fetch the remaining pages if any.
	if pageCount > 1 {
		if config.AppEnv.Global.EnableConcurrency {
			if err := req.fetchPagesConcurrently(req.ctx, cfg, totalCount, pageCount, pageModels); err != nil {
				logger.LogErr.Println("FETCH: Error fetching remaining pages! (" + err.Error() + ")")
				return err
			}
		} else {
			if err := req.fetchPagesSequentially(req.ctx, cfg, totalCount, pageCount, pageModels); err != nil {
				logger.LogErr.Println("FETCH: Error fetching remaining pages! (" + err.Error() + ")")
				return err
			}
		}
	}

	// Assemble all models in page order. Guard the capacity hint against a
	// malformed (negative) total count to avoid a make() panic.
	capacity := totalCount
	if capacity < 0 {
		capacity = 0
	}
	AllModels = make([]Model, 0, capacity)
	for page := 1; page <= pageCount; page++ {
		AllModels = append(AllModels, pageModels[page]...)
	}

	// The total number of fetched records must match the total count reported
	// by the API (taken from the first page). Abort on any mismatch.
	if len(AllModels) != totalCount {
		logger.LogErr.Println("FETCH: Total record count mismatch! (expected " + strconv.Itoa(totalCount) + ", got " + strconv.Itoa(len(AllModels)) + ")")
		return ErrTotalMismatch
	}

	logger.LogInfo.Println("FETCH: Total records fetched: " + strconv.Itoa(len(AllModels)))

	return nil

}
