package grpc

import (
	"context"
	"errors"
	"log/slog"
	"time"

	liveenginev1 "github.com/livelaw/backend/contracts/proto/v1"
	"github.com/livelaw/backend/internal/adapters/outbound/liveengine"
	"github.com/livelaw/backend/internal/domain/livecase"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// LiveEngineServer implements liveenginev1.LiveEngineServiceServer on top of MemoryLiveEngine.
type LiveEngineServer struct {
	liveenginev1.UnimplementedLiveEngineServiceServer
	memEngine *liveengine.MemoryLiveEngine
	logger    *slog.Logger
}

// NewLiveEngineServer creates a new LiveEngineServer instance.
func NewLiveEngineServer(memEngine *liveengine.MemoryLiveEngine, logger *slog.Logger) *LiveEngineServer {
	return &LiveEngineServer{
		memEngine: memEngine,
		logger:    logger,
	}
}

// Ingest accepts a live-update event and dispatches it into the memory engine.
func (s *LiveEngineServer) Ingest(ctx context.Context, req *liveenginev1.IngestRequest) (*liveenginev1.IngestResponse, error) {
	if req == nil || req.Event == nil {
		return nil, errors.New("event payload is required")
	}

	update := ProtoToLiveUpdate(req.Event)
	if err := s.memEngine.PublishUpdate(ctx, update); err != nil {
		return nil, err
	}

	return &liveenginev1.IngestResponse{
		Accepted:   true,
		SequenceNo: update.SequenceNo,
	}, nil
}

// Subscribe streams updates for a court case starting after sequence N.
func (s *LiveEngineServer) Subscribe(req *liveenginev1.SubscribeRequest, stream liveenginev1.LiveEngineService_SubscribeServer) error {
	if req == nil || req.CaseId == "" {
		return errors.New("case_id is required")
	}

	ctx := stream.Context()
	ch, err := s.memEngine.Subscribe(ctx, req.CaseId, req.AfterSequence)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update, ok := <-ch:
			if !ok {
				return nil
			}
			eventProto := LiveUpdateToProto(update)
			if err := stream.Send(eventProto); err != nil {
				s.logger.Warn("error streaming update to gRPC client",
					slog.String("case_id", req.CaseId),
					slog.Int64("sequence_no", update.SequenceNo),
					slog.Any("error", err),
				)
				return err
			}
		}
	}
}

// Snapshot returns the latest N updates and current highest sequence number for a court case.
func (s *LiveEngineServer) Snapshot(ctx context.Context, req *liveenginev1.SnapshotRequest) (*liveenginev1.SnapshotResponse, error) {
	if req == nil || req.CaseId == "" {
		return nil, errors.New("case_id is required")
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}

	highestSeq, updates := s.memEngine.Snapshot(req.CaseId, limit, req.AfterSequence)
	protoUpdates := make([]*liveenginev1.LiveUpdateEvent, len(updates))
	for i, u := range updates {
		protoUpdates[i] = LiveUpdateToProto(u)
	}

	return &liveenginev1.SnapshotResponse{
		CaseId:            req.CaseId,
		HighestSequenceNo: highestSeq,
		Updates:           protoUpdates,
	}, nil
}

// ProtoToLiveUpdate converts a Protobuf LiveUpdateEvent into a domain LiveUpdate model.
func ProtoToLiveUpdate(pe *liveenginev1.LiveUpdateEvent) livecase.LiveUpdate {
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

// LiveUpdateToProto converts a domain LiveUpdate model into a Protobuf LiveUpdateEvent.
func LiveUpdateToProto(u livecase.LiveUpdate) *liveenginev1.LiveUpdateEvent {
	return &liveenginev1.LiveUpdateEvent{
		SchemaVersion:    1,
		EventId:          u.ID,
		EventType:        "live_update." + string(u.Status),
		CaseId:           u.CourtCaseID,
		SequenceNo:       u.SequenceNo,
		UpdateId:         u.ID,
		UpdateType:       string(u.UpdateType),
		SpeakerLabel:     u.SpeakerLabel,
		Content:          u.Content,
		Status:           string(u.Status),
		CorrectsUpdateId: u.CorrectsUpdateID,
		EventTime:        timestamppb.New(u.EventTime),
		CreatedAt:        timestamppb.New(u.CreatedAt),
	}
}
