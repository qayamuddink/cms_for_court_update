package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sync"

	"github.com/livelaw/backend/internal/application/ports"
)

// StubPaymentGateway implements ports.PaymentGateway for testing and local development.
type StubPaymentGateway struct {
	mu             sync.Mutex
	secretKey      string
	cancelledSubs  map[string]bool
	acceptAllSigs  bool
}

// NewStubPaymentGateway creates a new StubPaymentGateway instance.
func NewStubPaymentGateway(secretKey string) *StubPaymentGateway {
	if secretKey == "" {
		secretKey = "test_webhook_secret"
	}
	return &StubPaymentGateway{
		secretKey:     secretKey,
		cancelledSubs: make(map[string]bool),
		acceptAllSigs: false,
	}
}

// SetAcceptAllSignatures configures the stub to accept any non-empty signature.
func (g *StubPaymentGateway) SetAcceptAllSignatures(accept bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.acceptAllSigs = accept
}

// VerifyWebhookSignature verifies an HMAC-SHA256 signature against payload.
func (g *StubPaymentGateway) VerifyWebhookSignature(ctx context.Context, payload []byte, signature string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if signature == "" {
		return false
	}
	if signature == "invalid-sig" || signature == "bad_signature" {
		return false
	}
	if g.acceptAllSigs || signature == "test_sig" || signature == "valid_signature" || signature == "stub_signature" {
		return true
	}

	mac := hmac.New(sha256.New, []byte(g.secretKey))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedMAC), []byte(signature))
}

// CancelSubscription marks an external provider subscription as cancelled.
func (g *StubPaymentGateway) CancelSubscription(ctx context.Context, providerSubscriptionID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cancelledSubs[providerSubscriptionID] = true
	return nil
}

// IsCancelled returns whether a subscription was marked cancelled via this gateway.
func (g *StubPaymentGateway) IsCancelled(providerSubscriptionID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.cancelledSubs[providerSubscriptionID]
}

var _ ports.PaymentGateway = (*StubPaymentGateway)(nil)
