package melu

import (
	"net/http"
)

type NewVersionResults struct {
	Version string
	Commit  string
}

type AuthTransport struct {
	Transport     http.RoundTripper
	Authorization string
}

func (t *AuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.Authorization != "" {
		req.Header.Set("Authorization", t.Authorization)
	}
	return t.Transport.RoundTrip(req)
}

func NewAuthTransport(tr http.RoundTripper, auth string) *AuthTransport {
	return &AuthTransport{
		Transport:     tr,
		Authorization: auth,
	}
}
