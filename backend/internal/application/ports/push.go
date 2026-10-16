package ports

import "context"

// PushMessage represents a mobile push notification.
type PushMessage struct {
	DeviceTokens []string
	Title        string
	Body         string
	Data         map[string]string
}

// PushSender defines push notification dispatch port.
type PushSender interface {
	SendBatch(ctx context.Context, message PushMessage) error
}
