package liveengine_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	liveenginev1 "github.com/livelaw/backend/contracts/proto/v1"
	grpcAdapter "github.com/livelaw/backend/internal/adapters/inbound/grpc"
	"github.com/livelaw/backend/internal/adapters/outbound/liveengine"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/domain/livecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func createTestUpdate(caseID string, seq int64, content string) livecase.LiveUpdate {
	now := time.Now().UTC()
	return livecase.LiveUpdate{
		ID:               fmt.Sprintf("update-%s-%d", caseID, seq),
		CourtCaseID:      caseID,
		SequenceNo:       seq,
		UpdateType:       livecase.UpdateTypeArgument,
		SpeakerLabel:     stringPtr(fmt.Sprintf("Counsel %d", seq)),
		Content:          content,
		ReporterID:       "reporter-uuid-1",
		Status:           livecase.UpdateStatusPublished,
		CorrectsUpdateID: nil,
		EventTime:        now,
		CreatedAt:        now,
	}
}

func stringPtr(s string) *string {
	return &s
}

// Contract suite executed identically against any ports.LiveEnginePort implementation.
func runLiveEngineContractSuite(t *testing.T, engine ports.LiveEnginePort) {
	ctx := context.Background()
	caseID := fmt.Sprintf("case-contract-%d", time.Now().UnixNano())

	// 1. Publish initial updates 1..5
	for seq := int64(1); seq <= 5; seq++ {
		u := createTestUpdate(caseID, seq, fmt.Sprintf("Argument point #%d", seq))
		if err := engine.PublishUpdate(ctx, u); err != nil {
			t.Fatalf("failed publishing update %d: %v", seq, err)
		}
	}

	// 2. Test GetLatestUpdates
	t.Run("GetLatestUpdates_Limit", func(t *testing.T) {
		latest, err := engine.GetLatestUpdates(ctx, caseID, 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(latest) != 3 {
			t.Fatalf("expected 3 latest updates, got %d", len(latest))
		}
		if latest[0].SequenceNo != 3 || latest[1].SequenceNo != 4 || latest[2].SequenceNo != 5 {
			t.Fatalf("unexpected sequence order for latest: %+v", latest)
		}
	})

	// 3. Test GetUpdatesAfter
	t.Run("GetUpdatesAfter_SequenceReplay", func(t *testing.T) {
		after2, err := engine.GetUpdatesAfter(ctx, caseID, 2, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(after2) != 3 {
			t.Fatalf("expected 3 updates after sequence 2, got %d", len(after2))
		}
		for i, expectedSeq := range []int64{3, 4, 5} {
			if after2[i].SequenceNo != expectedSeq {
				t.Fatalf("index %d expected seq %d, got %d", i, expectedSeq, after2[i].SequenceNo)
			}
		}

		after5, err := engine.GetUpdatesAfter(ctx, caseID, 5, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(after5) != 0 {
			t.Fatalf("expected 0 updates after sequence 5, got %d", len(after5))
		}
	})

	// 4. Reconnect Test (Mandatory Requirement): Subscribe from sequence K and receive exactly K+1 onward
	t.Run("Reconnect_SubscribeFromK_ReceiveKPlusOneOnward", func(t *testing.T) {
		// Populate case up to sequence 10
		for seq := int64(6); seq <= 10; seq++ {
			u := createTestUpdate(caseID, seq, fmt.Sprintf("Hearing update #%d", seq))
			if err := engine.PublishUpdate(ctx, u); err != nil {
				t.Fatalf("failed publishing update %d: %v", seq, err)
			}
		}

		// Client reconnects specifying K = 6 (after_sequence = 6)
		const K = int64(6)
		subCtx, subCancel := context.WithTimeout(ctx, 3*time.Second)
		defer subCancel()

		stream, err := engine.Subscribe(subCtx, caseID, K)
		if err != nil {
			t.Fatalf("failed to subscribe: %v", err)
		}

		// Must receive exactly K+1 (7) through 10 from replay
		expectedReplay := []int64{7, 8, 9, 10}
		for _, expectedSeq := range expectedReplay {
			select {
			case u, ok := <-stream:
				if !ok {
					t.Fatalf("stream closed prematurely waiting for seq %d", expectedSeq)
				}
				if u.SequenceNo != expectedSeq {
					t.Fatalf("reconnect error: expected sequence %d, got %d", expectedSeq, u.SequenceNo)
				}
			case <-time.After(1 * time.Second):
				t.Fatalf("timed out waiting for replayed sequence %d", expectedSeq)
			}
		}

		// Now publish a new live event (11) while subscriber is connected
		u11 := createTestUpdate(caseID, 11, "Live breakthrough announcement #11")
		if err := engine.PublishUpdate(ctx, u11); err != nil {
			t.Fatalf("failed publishing live update 11: %v", err)
		}

		select {
		case u, ok := <-stream:
			if !ok {
				t.Fatalf("stream closed prematurely waiting for live update 11")
			}
			if u.SequenceNo != 11 {
				t.Fatalf("expected live sequence 11, got %d", u.SequenceNo)
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("timed out waiting for live sequence 11")
		}
	})

	// 5. High-Concurrency Multi-Subscriber Fan-Out
	t.Run("MultiSubscriber_FanOut_Ordering", func(t *testing.T) {
		fanOutCaseID := fmt.Sprintf("case-fanout-%d", time.Now().UnixNano())
		subscriberCount := 10
		updateCount := 15

		subCtx, subCancel := context.WithTimeout(ctx, 5*time.Second)
		defer subCancel()

		var wg sync.WaitGroup
		wg.Add(subscriberCount)

		// Launch 10 concurrent subscribers starting from after_sequence = 0
		errChan := make(chan error, subscriberCount)
		for i := 0; i < subscriberCount; i++ {
			go func(subID int) {
				defer wg.Done()
				stream, err := engine.Subscribe(subCtx, fanOutCaseID, 0)
				if err != nil {
					errChan <- fmt.Errorf("subscriber %d failed to subscribe: %w", subID, err)
					return
				}

				for expectedSeq := int64(1); expectedSeq <= int64(updateCount); expectedSeq++ {
					select {
					case u, ok := <-stream:
						if !ok {
							errChan <- fmt.Errorf("subscriber %d stream closed at seq %d", subID, expectedSeq)
							return
						}
						if u.SequenceNo != expectedSeq {
							errChan <- fmt.Errorf("subscriber %d expected seq %d, got %d", subID, expectedSeq, u.SequenceNo)
							return
						}
					case <-time.After(3 * time.Second):
						errChan <- fmt.Errorf("subscriber %d timed out waiting for seq %d", subID, expectedSeq)
						return
					}
				}
			}(i)
		}

		// Small delay to ensure subscribers are registered before live publishing
		time.Sleep(50 * time.Millisecond)

		// Ingest 15 updates
		for seq := int64(1); seq <= int64(updateCount); seq++ {
			u := createTestUpdate(fanOutCaseID, seq, fmt.Sprintf("Concurrent fanout update %d", seq))
			if err := engine.PublishUpdate(ctx, u); err != nil {
				t.Fatalf("failed publishing fanout update %d: %v", seq, err)
			}
		}

		wg.Wait()
		close(errChan)

		for err := range errChan {
			if err != nil {
				t.Fatalf("fanout verification failed: %v", err)
			}
		}
	})
}

// TestLiveEngineContract_InProcess runs the contract suite against the in-process implementation.
func TestLiveEngineContract_InProcess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	memEngine := liveengine.NewMemoryLiveEngine(logger)
	inprocess := liveengine.NewInProcessLiveEngine(memEngine)

	t.Log("Running contract test suite against Mode: InProcess")
	runLiveEngineContractSuite(t, inprocess)
}

// TestLiveEngineContract_GRPC runs the identical contract suite against the gRPC client/server implementation.
func TestLiveEngineContract_GRPC(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	memEngine := liveengine.NewMemoryLiveEngine(logger)

	// Set up in-memory gRPC server via bufconn
	bufferSize := 1024 * 1024
	lis := bufconn.Listen(bufferSize)
	grpcServer := grpc.NewServer()
	serverAdapter := grpcAdapter.NewLiveEngineServer(memEngine, logger)
	liveenginev1.RegisterLiveEngineServiceServer(grpcServer, serverAdapter)

	go func() {
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			t.Errorf("gRPC server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	// Connect gRPC client via bufconn dialer
	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial bufnet gRPC server: %v", err)
	}
	defer conn.Close()

	grpcClient := liveengine.NewGRPCLiveEngineClient(conn, logger)

	t.Log("Running contract test suite against Mode: gRPC")
	runLiveEngineContractSuite(t, grpcClient)
}
