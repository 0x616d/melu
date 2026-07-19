package rlhttp

import (
	"net/http"

	"golang.org/x/time/rate"
)

// RLHTTPClient Rate Limited HTTP Client
type RLHTTPClient struct {
	Client      *http.Client
	Ratelimiter *rate.Limiter
}

// Do dispatches the HTTP request to the network
func (c *RLHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if c.Ratelimiter != nil {
		if err := c.Ratelimiter.Wait(req.Context()); err != nil {
			return nil, err
		}
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// NewClient return rate_limited_http client with a ratelimiter
func NewClient(rl *rate.Limiter) *RLHTTPClient {
	c := &RLHTTPClient{
		Client:      http.DefaultClient,
		Ratelimiter: rl,
	}
	return c
}
