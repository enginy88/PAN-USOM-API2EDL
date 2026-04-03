package usom

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/enginy88/PAN-USOM-API2EDL/config"
	"github.com/enginy88/PAN-USOM-API2EDL/logger"

	"github.com/go-resty/resty/v2"
)

var (
	ErrEmptyAPIPath   = errors.New("empty api path")
	ErrFailedResponse = errors.New("response with non 2xx status code")
	ErrTypeAssert     = errors.New("type assertion failure")
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

func (req *APIRequest) fetchPage(page int, config *Config) (*Response, error) {

	params := map[string]string{
		"page":     strconv.Itoa(page),
		"per-page": "99999", // Always set per-page to 99999
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

	r := req.client.R().SetContext(req.ctx)
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

	logger.LogInfo.Println("FETCH: API call succeed with status: " + resp.Status() + " (PROTO: " + resp.Proto() + " HEADER: " + headerToString(resp.Header()) + ")")

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

func FetchAllPages(ctx context.Context) error {

	req, err := newAPIRequest(ctx, config.AppEnv.Global.APIPath)
	if err != nil {
		logger.LogErr.Println("FETCH: Failed to API Fetcher! (" + err.Error() + ")")
		return err
	}

	defer req.cancelRequest()

	config := &Config{
		PerPage: 99999,
	}

	// Fetch first page to get total page count
	firstPage, err := req.fetchPage(1, config)
	if err != nil {
		logger.LogErr.Println("FETCH: Error fetching first page! (" + err.Error() + ")")
		return err
	}

	// Initialize slice to hold all models
	AllModels = make([]Model, 0, firstPage.TotalCount)
	AllModels = append(AllModels, firstPage.Models...)

	logger.LogInfo.Println("FETCH: Fetched page 1/" + strconv.Itoa(firstPage.PageCount) + ", got " + strconv.Itoa(len(firstPage.Models)) + " records.")

	// Fetch remaining pages if any
	for page := 2; page <= firstPage.PageCount; page++ {
		resp, err := req.fetchPage(page, config)
		if err != nil {
			logger.LogErr.Println("FETCH: Error fetching the page: " + strconv.Itoa(page) + "! (" + err.Error() + ")")
			return err
		}
		AllModels = append(AllModels, resp.Models...)
		logger.LogInfo.Println("FETCH: Fetched page " + strconv.Itoa(page) + "/" + strconv.Itoa(firstPage.PageCount) + ", got " + strconv.Itoa(len(resp.Models)) + " records.")

	}

	logger.LogInfo.Println("FETCH: Total records fetched: " + strconv.Itoa(len(AllModels)))

	return nil

}
