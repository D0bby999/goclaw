package httpclient

import "net/http"

// Response holds the result of an HTTP request.
type Response struct {
	StatusCode  int
	Body        string
	Headers     http.Header
	ContentType string
	URL         string // final URL after redirects
}
