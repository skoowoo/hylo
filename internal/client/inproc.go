package client

import (
	"net/http"
	"net/http/httptest"
	"time"
)

// NewInProcess returns a Client that calls h directly instead of going over TCP.
// Requests skip the server's auth middleware, so h must be the inner router.
func NewInProcess(h http.Handler) *Client {
	return &Client{
		http:    &http.Client{Transport: handlerTransport{h}, Timeout: 30 * time.Second},
		baseURL: "http://hylo.inproc",
	}
}

type handlerTransport struct{ h http.Handler }

func (t handlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	t.h.ServeHTTP(rec, req)
	return rec.Result(), nil
}
