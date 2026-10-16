package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/livelaw/backend/internal/application/ports"
)

type outboxCaseUpdate struct {
	CaseID     string  `json:"case_id"`
	UpdateID   string  `json:"update_id"`
	UpdateType string  `json:"update_type"`
	Content    string  `json:"content"`
	Status     string  `json:"status"`
}

// PushDispatcher consumes outbox events and dispatches push notifications to case followers.
type PushDispatcher struct {
	followRepo ports.CaseFollowRepository
	deviceRepo ports.DeviceTokenRepository
	caseRepo   ports.CourtCaseRepository
	pushSender ports.PushSender
	logger     *slog.Logger
}

// NewPushDispatcher creates a new PushDispatcher instance.
func NewPushDispatcher(
	followRepo ports.CaseFollowRepository,
	deviceRepo ports.DeviceTokenRepository,
	caseRepo ports.CourtCaseRepository,
	pushSender ports.PushSender,
	logger *slog.Logger,
) *PushDispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &PushDispatcher{
		followRepo: followRepo,
		deviceRepo: deviceRepo,
		caseRepo:   caseRepo,
		pushSender: pushSender,
		logger:     logger,
	}
}

// Publish inspects the event and triggers push alerts for verdict and order updates.
func (d *PushDispatcher) Publish(ctx context.Context, event ports.OutboxEvent) error {
	if event.AggregateType != "court_case" {
		return nil
	}

	var payload outboxCaseUpdate
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		d.logger.Error("failed unmarshaling outbox payload for push notification",
			slog.String("event_id", event.ID),
			slog.Any("error", err),
		)
		return nil
	}

	updateType := strings.ToLower(payload.UpdateType)
	// We only dispatch push notifications for verdict and order updates
	if updateType != "verdict" && updateType != "order" {
		return nil
	}

	followers, err := d.followRepo.GetFollowersByCaseID(ctx, payload.CaseID, updateType)
	if err != nil {
		d.logger.Error("failed retrieving case followers",
			slog.String("case_id", payload.CaseID),
			slog.Any("error", err),
		)
		return err
	}

	if len(followers) == 0 {
		return nil
	}

	tokens, err := d.deviceRepo.GetTokensByUserIDs(ctx, followers)
	if err != nil {
		d.logger.Error("failed querying device tokens for followers",
			slog.Int("follower_count", len(followers)),
			slog.Any("error", err),
		)
		return err
	}

	if len(tokens) == 0 {
		return nil
	}

	var rawTokens []string
	for _, t := range tokens {
		rawTokens = append(rawTokens, t.PushToken)
	}

	caseTitle := "Court Update"
	if d.caseRepo != nil {
		c, err := d.caseRepo.GetByID(ctx, payload.CaseID)
		if err == nil && c != nil {
			caseTitle = c.Title
		}
	}

	msg := ports.PushMessage{
		DeviceTokens: rawTokens,
		Title:        fmt.Sprintf("[%s] %s", strings.ToUpper(updateType), caseTitle),
		Body:         payload.Content,
		Data: map[string]string{
			"case_id":     payload.CaseID,
			"update_id":   payload.UpdateID,
			"update_type": updateType,
		},
	}

	if err := d.pushSender.SendBatch(ctx, msg); err != nil {
		d.logger.Error("failed dispatching push notifications to followers",
			slog.String("case_id", payload.CaseID),
			slog.Int("recipient_count", len(rawTokens)),
			slog.Any("error", err),
		)
		return err
	}

	d.logger.Info("push notifications successfully dispatched",
		slog.String("case_id", payload.CaseID),
		slog.String("update_type", updateType),
		slog.Int("recipients", len(rawTokens)),
	)

	return nil
}

// CompositeEventPublisher broadcasts an outbox event to multiple publishers.
type CompositeEventPublisher struct {
	publishers []ports.EventPublisher
}

// NewCompositeEventPublisher combines multiple EventPublishers into one.
func NewCompositeEventPublisher(publishers ...ports.EventPublisher) *CompositeEventPublisher {
	return &CompositeEventPublisher{publishers: publishers}
}

// Publish dispatches the event to all registered publishers sequentially.
func (c *CompositeEventPublisher) Publish(ctx context.Context, event ports.OutboxEvent) error {
	for _, p := range c.publishers {
		if p == nil {
			continue
		}
		if err := p.Publish(ctx, event); err != nil {
			return err
		}
	}
	return nil
}
