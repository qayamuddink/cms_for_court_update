package push

import (
	"context"
	"log/slog"
	"sync"

	"github.com/livelaw/backend/internal/application/ports"
)

// StubPushSender implements ports.PushSender for testing and development.
type StubPushSender struct {
	mu           sync.RWMutex
	logger       *slog.Logger
	sentMessages []ports.PushMessage
}

// NewStubPushSender creates a new StubPushSender.
func NewStubPushSender(logger *slog.Logger) *StubPushSender {
	if logger == nil {
		logger = slog.Default()
	}
	return &StubPushSender{
		logger:       logger,
		sentMessages: make([]ports.PushMessage, 0),
	}
}

// SendBatch logs and captures outgoing push notifications.
func (s *StubPushSender) SendBatch(ctx context.Context, message ports.PushMessage) error {
	s.mu.Lock()
	s.sentMessages = append(s.sentMessages, message)
	s.mu.Unlock()

	s.logger.Info("push notification dispatched",
		slog.Int("recipient_tokens", len(message.DeviceTokens)),
		slog.String("title", message.Title),
		slog.String("body", message.Body),
		slog.Any("data", message.Data),
	)

	return nil
}

// GetSentMessages returns a copy of all sent push notifications.
func (s *StubPushSender) GetSentMessages() []ports.PushMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copied := make([]ports.PushMessage, len(s.sentMessages))
	copy(copied, s.sentMessages)
	return copied
}

// LastMessage returns the most recent sent push message, or nil if none sent.
func (s *StubPushSender) LastMessage() *ports.PushMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.sentMessages) == 0 {
		return nil
	}
	msg := s.sentMessages[len(s.sentMessages)-1]
	return &msg
}

// Reset clears the captured messages.
func (s *StubPushSender) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sentMessages = make([]ports.PushMessage, 0)
}

var _ ports.PushSender = (*StubPushSender)(nil)
