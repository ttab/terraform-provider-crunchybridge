/*
Copyright 2021 Crunchy Data Solutions, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package bridgeapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/uuid"
)

const (
	routeAccount        = "/account"
	routeCluster        = "/clusters/%s"
	routeClusterUpgrade = "/clusters/%s/upgrade"
	routeClusters       = "/clusters"
	routeClusterRole    = "/clusters/%s/roles/%s"
	routeClusterStatus  = "/clusters/%s/status"
	routeProviders      = "/providers"
	routeTeams          = "/teams"
)

var BridgeProviderNS = uuid.MustParse("cc67b0e5-7152-4d54-85ff-49a5c17fbbfe")

type ClientOption func(*Client) error

type Client struct {
	apiTarget         *url.URL
	client            *http.Client
	credential        TokenSource
	useIdempotencyKey bool
	userAgent         string
}

func NewClient(apiURL *url.URL, cred TokenSource, opts ...ClientOption) (*Client, error) {
	if apiURL == nil {
		return nil, errors.New("cannot create client to nil URL target")
	}

	// Defaults unless overridden by options
	c := &Client{
		apiTarget:  apiURL,
		client:     &http.Client{},
		credential: cred,
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("error during client initialization: %w", err)
		}
	}

	return c, nil
}

// WithHTTPClient allows the use of a custom-configured HTTP client for API
// requests, Client defaults to a default http.Client{} otherwise
// Setter - always returns nil error
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) error {
		c.client = hc
		return nil
	}
}

// WithUserAgent configures a UserAgent string to use in all requests to the API
// Setter - always returns nil error
func WithUserAgent(ua string) ClientOption {
	return func(c *Client) error {
		c.userAgent = ua
		return nil
	}
}

// WithImmediateLogin triggers a login instead of waiting for lazy-initialization
// to occcur once a data function is called
func WithImmediateLogin() ClientOption {
	return func(c *Client) error {
		_, err := c.credential.GetToken(context.Background(), c)
		return err
	}
}

// WithIdempotencyKey causes the client to send an Idempotency Key header on cluster create
// N.B. This may have unexpected behavior tied to cached responses after system state
// changes invalidate the correctness of those responses
func WithIdempotencyKey() ClientOption {
	return func(c *Client) error {
		c.useIdempotencyKey = true
		return nil
	}
}

// Close allows an explicit request to log out of the current session
// There is no explicit login, as that's handled transparently before API calls.
func (c *Client) Close(ctx context.Context) error {
	// Right now, needs nothing more than invalidating the access token
	return c.credential.Close(ctx, c)
}

// helper to ensure headers used in all requests are set consistently
func (c *Client) setCommonHeaders(req *http.Request) error {
	token, err := c.credential.GetToken(req.Context(), c)
	if err != nil {
		return fmt.Errorf(
			"failed to get token for request authentication: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	return nil
}

func (c *Client) resolve(path string, params ...url.Values) *url.URL {
	u := c.apiTarget.ResolveReference(&url.URL{Path: path})

	q := u.Query()

	for _, p := range params {
		for name, value := range p {
			q[name] = value
		}
	}

	u.RawQuery = q.Encode()

	return u
}
