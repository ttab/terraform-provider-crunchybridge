/*
Copyright 2022 Crunchy Data Solutions, Inc.

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
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type TokenSource interface {
	GetToken(ctx context.Context, c *Client) (string, error)
	Close(ctx context.Context, c *Client) error
}

const (
	// Renew token if it only has 1 minute until expiry, this should
	// eliminate use of exired tokens due to network lag or clock jitter.
	minTimeUntilExpiry = 1 * time.Minute
	// Don't bother deleting tokens that are about to expire.
	minTimeLeftForDelete = 1 * time.Minute
)

type LegacyAuth struct {
	id     string
	secret string

	m       sync.Mutex
	tokenID string
	token   string
	expires time.Time
}

func NewLegacyAuth(id, secret string) *LegacyAuth {
	return &LegacyAuth{
		id:     id,
		secret: secret,
	}
}

func (a *LegacyAuth) GetToken(ctx context.Context, c *Client) (_ string, outErr error) {
	a.m.Lock()
	defer a.m.Unlock()

	if time.Until(a.expires) > minTimeUntilExpiry {
		return a.token, nil
	}

	route := c.resolve("/access-tokens")

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, route.String(), nil,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to perform request: %w", err)
	}

	defer safeClose(&outErr, resp.Body, "response body")

	if resp.StatusCode != http.StatusOK {
		return "", errorFromAPIMessageResponse(resp)
	}

	var tr struct {
		ExpiresIn int64  `json:"expires_in"`
		Token     string `json:"access_token"`
		TokenID   string `json:"id"`
	}

	err = json.NewDecoder(resp.Body).Decode(&tr)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	a.tokenID = tr.TokenID
	a.token = tr.Token
	a.expires = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)

	return tr.Token, nil
}

func (a *LegacyAuth) Close(ctx context.Context, c *Client) (outErr error) {
	a.m.Lock()
	defer a.m.Unlock()

	if a.tokenID == "" || time.Until(a.expires) < minTimeLeftForDelete {
		return nil
	}

	route := c.resolve(fmt.Sprintf("/access-tokens/%s", a.tokenID))

	req, err := http.NewRequestWithContext(
		ctx, http.MethodDelete, route.String(), nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform request: %w", err)
	}

	defer safeClose(&outErr, resp.Body, "response body")

	if resp.StatusCode != http.StatusOK {
		return errorFromAPIMessageResponse(resp)
	}

	a.tokenID = ""
	a.token = ""
	a.expires = time.Time{}

	return nil
}

type APIKeyAuth string

func (a APIKeyAuth) GetToken(ctx context.Context, c *Client) (string, error) {
	return string(a), nil
}

func (a APIKeyAuth) Close(ctx context.Context, c *Client) error {
	return nil
}
