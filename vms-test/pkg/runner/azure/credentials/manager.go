package credentials

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// Manager manages Azure credentials with background token refresh
type Manager struct {
	credential azcore.TokenCredential
	mu         sync.RWMutex
}

// NewManager creates a new credential manager with background token refresh
func NewManager(ctx context.Context) (*Manager, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}

	m := &Manager{
		credential: cred,
	}

	// Start background token refresh
	go m.refreshLoop(ctx)

	return m, nil
}

// GetCredential returns the current credential
func (m *Manager) GetCredential() azcore.TokenCredential {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.credential
}

// refreshLoop runs in the background and refreshes tokens every 4 minutes
func (m *Manager) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(4 * time.Minute)
	defer ticker.Stop()

	// Attempt initial token refresh to validate credentials
	m.refreshToken(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Azure credential manager: shutting down token refresh")
			return
		case <-ticker.C:
			m.refreshToken(ctx)
		}
	}
}

// refreshToken attempts to refresh the token by calling GetToken
func (m *Manager) refreshToken(ctx context.Context) {
	// Use the standard Azure resource manager scope
	scopes := []string{"https://management.azure.com/.default"}

	options := policy.TokenRequestOptions{
		Scopes: scopes,
	}

	m.mu.Lock()
	cred := m.credential
	m.mu.Unlock()

	token, err := cred.GetToken(ctx, options)
	if err != nil {
		log.Printf("Azure credential manager: failed to refresh token: %v", err)
		// Don't exit - continue trying to refresh on next interval
		return
	}

	log.Printf("Azure credential manager: successfully refreshed token (expires: %v)", token.ExpiresOn)
}

// NewDefaultCredential creates a new Azure credential with background token refresh.
// This is a drop-in replacement for azidentity.NewDefaultAzureCredential that includes
// automatic token refresh to prevent timeout issues.
func NewDefaultCredential(ctx context.Context) (azcore.TokenCredential, error) {
	mgr, err := NewManager(ctx)
	if err != nil {
		return nil, err
	}
	return mgr.GetCredential(), nil
}
