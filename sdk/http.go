package sdk

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// HTTPRequest defines a proxied HTTP request.
type HTTPRequest struct {
	Method     string
	URL        string
	Headers    map[string]string
	Body       []byte
	BodyBase64 bool
	TimeoutMS  int
}

// HTTPResponse contains the proxied response data.
type HTTPResponse struct {
	Status   int
	Headers  map[string]string
	Body     []byte
	Duration time.Duration
}

type httpRequestPayload struct {
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	BodyBase64 string            `json:"body_base64,omitempty"`
	TimeoutMS  int               `json:"timeout_ms,omitempty"`
}

type httpResponsePayload struct {
	Status       int               `json:"status"`
	Headers      map[string]string `json:"headers,omitempty"`
	BodyBase64   string            `json:"body_base64"`
	BodyEncoding string            `json:"body_encoding,omitempty"`
}

// HTTP provides helper methods for host-proxied HTTP requests.
//
//nolint:gochecknoglobals
var HTTP = &HTTPClient{MaxResponseBytes: MaxHTTPResponseBytes}

// HTTPClient wraps host HTTP request calls.
type HTTPClient struct {
	MaxResponseBytes uint32
}

// Do performs an HTTP request via the host proxy.
func (c *HTTPClient) Do(req HTTPRequest) (*HTTPResponse, error) {
	payload := httpRequestPayload{
		Method:    strings.ToUpper(strings.TrimSpace(req.Method)),
		URL:       req.URL,
		Headers:   req.Headers,
		TimeoutMS: req.TimeoutMS,
	}
	if payload.Method == "" {
		payload.Method = "GET"
	}
	if len(req.Body) > 0 {
		if req.BodyBase64 {
			payload.BodyBase64 = base64.StdEncoding.EncodeToString(req.Body)
		} else {
			payload.Body = string(req.Body)
		}
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	respBufSize := c.MaxResponseBytes
	if respBufSize == 0 {
		respBufSize = MaxHTTPResponseBytes
	}
	respBuf := make([]byte, respBufSize)
	start := time.Now()
	res := hostHTTPRequest(ptrFromBytes(encoded), uint32(len(encoded)), ptrFromBytes(respBuf), uint32(len(respBuf)))
	if err := hostErr(res, "http_request"); err != nil {
		return nil, err
	}
	if res == 0 {
		return &HTTPResponse{Duration: time.Since(start)}, nil
	}
	if uint32(res) > uint32(len(respBuf)) {
		return nil, HostError{Code: hostErrTooLarge, Op: "http_request"}
	}

	var responsePayload httpResponsePayload
	if err := json.Unmarshal(respBuf[:res], &responsePayload); err != nil {
		return nil, err
	}

	body := []byte{}
	if responsePayload.BodyBase64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(responsePayload.BodyBase64)
		if err != nil {
			return nil, err
		}
		body = decoded
	}

	return &HTTPResponse{
		Status:   responsePayload.Status,
		Headers:  responsePayload.Headers,
		Body:     body,
		Duration: time.Since(start),
	}, nil
}

// Get performs a GET request.
func (c *HTTPClient) Get(url string) (*HTTPResponse, error) {
	return c.Do(HTTPRequest{Method: "GET", URL: url})
}

// Post performs a POST request with the provided body.
func (c *HTTPClient) Post(url string, body []byte, contentType string) (*HTTPResponse, error) {
	headers := map[string]string{}
	if contentType != "" {
		headers["content-type"] = contentType
	}
	return c.Do(HTTPRequest{Method: "POST", URL: url, Headers: headers, Body: body})
}
