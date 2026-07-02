package github

import (
	"context"
	"sync"
	"time"
)

// InstallationTokenCache caches a GitHub App installation access token. Installation
// tokens are valid for 60 minutes; we refresh proactively at 55 minutes so in-flight
// requests never hit an expired token.
//
// In a single-org deployment there is exactly one installation, so one cache entry
// is sufficient. Multiple goroutines calling GetOrRefresh concurrently will serialize
// on the mutex — only the first caller triggers a token exchange.
type InstallationTokenCache struct {
	mu      sync.Mutex
	client  Client
	expires time.Time
}

func NewInstallationTokenCache() *InstallationTokenCache {
	return &InstallationTokenCache{}
}

// GetOrRefresh returns the cached Client if it is still valid, otherwise exchanges
// a new installation access token with GitHub and caches the result.
func (c *InstallationTokenCache) GetOrRefresh(ctx context.Context, appID int64, privateKeyPEM []byte, installationID int64) (Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client != nil && time.Now().Before(c.expires) {
		return c.client, nil
	}
	client, err := NewInstallationClient(ctx, appID, privateKeyPEM, installationID)
	if err != nil {
		return nil, err
	}
	c.client = client
	c.expires = time.Now().Add(55 * time.Minute)
	return client, nil
}
