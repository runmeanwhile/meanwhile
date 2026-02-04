package mcp

import (
	"net/http"
)

type headerRoundTripper struct {
	base    http.RoundTripper
	headers http.Header
}

func (h headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	for key, values := range h.headers {
		for _, value := range values {
			clone.Header.Add(key, value)
		}
	}
	return h.base.RoundTrip(clone)
}

func withHeaders(client *http.Client, headers http.Header) *http.Client {
	if len(headers) == 0 {
		return client
	}

	base := http.DefaultTransport
	if client != nil && client.Transport != nil {
		base = client.Transport
	}
	wrapped := &http.Client{}
	if client != nil {
		*wrapped = *client
	}
	wrapped.Transport = headerRoundTripper{base: base, headers: headers.Clone()}
	return wrapped
}
