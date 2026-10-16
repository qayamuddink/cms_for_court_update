package liveengine

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	liveenginev1 "github.com/livelaw/backend/contracts/proto/v1"
	"github.com/livelaw/backend/internal/domain/livecase"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCLiveEngineClient implements ports.LiveEnginePort by communicating over gRPC.
type GRPCLiveEngineClient struct {
	client liveenginev1.LiveEngineServiceClient
	conn   *grpc.ClientConn
	logger *slog.Logger
}

// NewGRPCLiveEngineClient creates a new GRPCLiveEngineClient connected to the specified gRPC address.
func NewGRPCLiveEngineClient(conn *grpc.ClientConn, logger *slog.Logger) *GRPCLiveEngineClient {
	return &GRPCLiveEngineClient{
		client: liveenginev1.NewLiveEngineServiceClient(conn),
		conn:   conn,
		logger: logger,
	}
}

// Close closes the underlying gRPC client connection.
func (c *GRPCLiveEngineClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// PublishUpdate sends an update event to the live engine via Ingest RPC.
func (c *GRPCLiveEngineClient) PublishUpdate(ctx context.Context, update livecase.LiveUpdate) error {
	eventProto := &liveenginev1.LiveUpdateEvent{
		SchemaVersion:    1,
		EventId:          update.ID,
		EventType:        "live_update." + string(update.Status),
		CaseId:           update.CourtCaseID,
		SequenceNo:       update.SequenceNo,
		UpdateId:         update.ID,
		UpdateType:       string(update.UpdateType),
		SpeakerLabel:     update.SpeakerLabel,
		Content:          update.Content,
		Status:           string(update.Status),
		CorrectsUpdateId: update.CorrectsUpdateID,
		EventTime:        timestamppb.New(update.EventTime),
		CreatedAt:        timestamppb.New(update.CreatedAt),
	}

	resp, err := c.client.Ingest(ctx, &liveenginev1.IngestRequest{Event: eventProto})
	if err != nil {
		return fmt.Errorf("failed to ingest update to live engine over gRPC: %w", err)
	}

	if !resp.Accepted {
		return fmt.Errorf("live engine rejected update event %s (sequence %d)", update.ID, update.SequenceNo)
	}

	return nil
}

// GetUpdatesAfter queries the live engine for updates strictly after afterSequence.
func (c *GRPCLiveEngineClient) GetUpdatesAfter(ctx context.Context, caseID string, afterSequence int64, limit int) ([]livecase.LiveUpdate, error) {
	resp, err := c.client.Snapshot(ctx, &liveenginev1.SnapshotRequest{
		CaseId:        caseID,
		Limit:         int32(limit),
		AfterSequence: &afterSequence,
	})
	if err != nil {
		return nil, fmt.Errorf("failed querying updates after sequence over gRPC: %w", err)
	}

	updates := make([]livecase.LiveUpdate, len(resp.Updates))
	for i, pe := range resp.Updates {
		updates[i] = protoToLiveUpdate(pe)
	}
	return updates, nil
}

// GetLatestUpdates queries the live engine for the most recent updates.
func (c *GRPCLiveEngineClient) GetLatestUpdates(ctx context.Context, caseID string, limit int) ([]livecase.LiveUpdate, error) {
	resp, err := c.client.Snapshot(ctx, &liveenginev1.SnapshotRequest{
		CaseId: caseID,
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed querying latest updates over gRPC: %w", err)
	}

	updates := make([]livecase.LiveUpdate, len(resp.Updates))
	for i, pe := range resp.Updates {
		updates[i] = protoToLiveUpdate(pe)
	}
	return updates, nil
}

// Subscribe streams updates from sequence afterSequence over a gRPC server stream.
func (c *GRPCLiveEngineClient) Subscribe(ctx context.Context, caseID string, afterSequence int64) (<-chan livecase.LiveUpdate, error) {
	stream, err := c.client.Subscribe(ctx, &liveenginev1.SubscribeRequest{
		CaseId:        caseID,
		AfterSequence: afterSequence,
	})
	if err != nil {
		return nil, fmt.Errorf("failed opening gRPC subscribe stream: %w", err)
	}

	ch := make(chan livecase.LiveUpdate, 200)

	go func() {
		defer close(ch)
		for {
			event, err := stream.Recv()
			if err != nil {
				if err != io.EOF && ctx.Err() == nil {
					c.logger.Warn("gRPC subscribe stream ended with error",
						slog.String("case_id", caseID),
						slog.Any("error", err),
					)
				}
				return
			}

			update := protoToLiveUpdate(event)
			select {
			case <-ctx.Done():
				return
			case ch <- update:
			}
		}
	}()

	return ch, nil
}

func protoToLiveUpdate(pe *liveenginev1.LiveUpdateEvent) livecase.LiveUpdate {
	if pe == nil {
		return livecase.LiveUpdate{}
	}
	var eventTime, createdAt time.Time
	if pe.EventTime != nil {
		eventTime = pe.EventTime.AsTime()
	}
	if pe.CreatedAt != nil {
		createdAt = pe.CreatedAt.AsTime()
	}
	return livecase.LiveUpdate{
		ID:               pe.UpdateId,
		CourtCaseID:      pe.CaseId,
		SequenceNo:       pe.SequenceNo,
		UpdateType:       livecase.UpdateType(pe.UpdateType),
		SpeakerLabel:     pe.SpeakerLabel,
		Content:          pe.Content,
		Status:           livecase.UpdateStatus(pe.Status),
		CorrectsUpdateID: pe.CorrectsUpdateId,
		EventTime:        eventTime,
		CreatedAt:        createdAt,
	}
}
