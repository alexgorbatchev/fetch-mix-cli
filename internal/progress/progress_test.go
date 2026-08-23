package progress_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/progress"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		input      string
		wantScheme string
		wantErr    bool
	}{
		{"unix:///tmp/test.sock", "unix", false},
		{"/tmp/test.sock", "unix", false},
		{"./test.sock", "unix", false},
		{"tcp://127.0.0.1:9099", "tcp", false},
		{"127.0.0.1:9099", "tcp", false},
		{"fd://3", "fd", false},
		{"stdout", "stdout", false},
		{"stderr", "stderr", false},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			scheme, _, err := progress.ParseTarget(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseTarget(%q) err = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && scheme != tt.wantScheme {
				t.Errorf("ParseTarget(%q) scheme = %q, want %q", tt.input, scheme, tt.wantScheme)
			}
		})
	}
}

func TestNewReporter_UnixSocket(t *testing.T) {
	sockPath := filepath.Join(os.TempDir(), fmt.Sprintf("test_rep_%d.sock", time.Now().UnixNano()))
	defer os.Remove(sockPath)

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Listen failed: %v", err)
	}
	defer listener.Close()

	var receivedEvents []progress.Event
	var mu sync.Mutex
	done := make(chan struct{})

	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		decoder := progress.NewDecoder(conn)
		for {
			var ev progress.Event
			if err := decoder.Decode(&ev); err != nil {
				break
			}
			mu.Lock()
			receivedEvents = append(receivedEvents, ev)
			mu.Unlock()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	reporter, err := progress.NewReporter(ctx, sockPath)
	if err != nil {
		t.Fatalf("NewReporter failed: %v", err)
	}

	event := progress.Event{
		Type:    progress.EventPhaseStart,
		Phase:   "download",
		Step:    3,
		Message: "downloading track",
	}

	if err := reporter.Emit(event); err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	_ = reporter.Close()

	<-done

	mu.Lock()
	defer mu.Unlock()
	if len(receivedEvents) != 1 {
		t.Fatalf("expected 1 event, got %d", len(receivedEvents))
	}
	if receivedEvents[0].Phase != "download" || receivedEvents[0].Type != progress.EventPhaseStart {
		t.Errorf("unexpected event: %+v", receivedEvents[0])
	}
}

func TestStartSocketServer_EventStreaming(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var received []progress.Event
	var mu sync.Mutex
	eventReceived := make(chan struct{}, 2)

	server, targetPath, err := progress.StartSocketServer(ctx, func(ev progress.Event) {
		mu.Lock()
		received = append(received, ev)
		mu.Unlock()
		eventReceived <- struct{}{}
	})
	if err != nil {
		t.Fatalf("StartSocketServer failed: %v", err)
	}
	defer server.Close()

	if targetPath == "" {
		t.Fatalf("expected non-empty targetPath")
	}

	reporter, err := progress.NewReporter(ctx, targetPath)
	if err != nil {
		t.Fatalf("NewReporter connecting to server failed: %v", err)
	}

	ev1 := progress.Event{
		Type:  progress.EventCandidateSelected,
		Phase: "search",
		Candidate: &progress.CandidateInfo{
			Title:  "Artist - Track (Extended Mix)",
			Source: "soundcloud",
			Score:  150,
		},
	}
	ev2 := progress.Event{
		Type:  progress.EventComplete,
		Phase: "complete",
		Result: &progress.ResultInfo{
			Path:            "tracks/01 - Artist - Track.m4a",
			Title:           "Track",
			Artist:          "Artist",
			BandwidthHz:     20000,
			BandwidthRating: "High Fidelity (>=18.5 kHz)",
			SuggestedGainDb: -2.4,
		},
	}

	if err := reporter.Emit(ev1); err != nil {
		t.Fatalf("Emit ev1 failed: %v", err)
	}
	if err := reporter.Emit(ev2); err != nil {
		t.Fatalf("Emit ev2 failed: %v", err)
	}
	_ = reporter.Close()

	for i := 0; i < 2; i++ {
		select {
		case <-eventReceived:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for events")
		}
	}

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 2 {
		t.Fatalf("expected 2 received events, got %d", len(received))
	}
	if received[0].Type != progress.EventCandidateSelected || received[0].Candidate.Title != "Artist - Track (Extended Mix)" {
		t.Errorf("unexpected event 0: %+v", received[0])
	}
	if received[1].Type != progress.EventComplete || received[1].Result.BandwidthHz != 20000 {
		t.Errorf("unexpected event 1: %+v", received[1])
	}
}

func TestNewReporter_TCPSocket(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on tcp: %v", err)
	}
	defer listener.Close()

	var receivedEvents []progress.Event
	var mu sync.Mutex
	done := make(chan struct{})

	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		decoder := progress.NewDecoder(conn)
		var ev progress.Event
		if err := decoder.Decode(&ev); err == nil {
			mu.Lock()
			receivedEvents = append(receivedEvents, ev)
			mu.Unlock()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	addr := fmt.Sprintf("tcp://%s", listener.Addr().String())
	reporter, err := progress.NewReporter(ctx, addr)
	if err != nil {
		t.Fatalf("NewReporter failed: %v", err)
	}

	if err := reporter.Emit(progress.Event{
		Type:    progress.EventComplete,
		Message: "all done",
		Result: &progress.ResultInfo{
			Path:  "tracks/test.m4a",
			Title: "Test",
		},
	}); err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	if err := reporter.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	<-done

	mu.Lock()
	defer mu.Unlock()
	if len(receivedEvents) != 1 {
		t.Fatalf("expected 1 event, got %d", len(receivedEvents))
	}
	if receivedEvents[0].Type != progress.EventComplete || receivedEvents[0].Result.Path != "tracks/test.m4a" {
		t.Errorf("unexpected event: %+v", receivedEvents[0])
	}
}

func TestNewReporter_FD(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	defer r.Close()

	ctx := context.Background()
	target := fmt.Sprintf("fd://%d", w.Fd())
	reporter, err := progress.NewReporter(ctx, target)
	if err != nil {
		w.Close()
		t.Fatalf("NewReporter with fd failed: %v", err)
	}

	ev := progress.Event{
		Type:    progress.EventProgress,
		Message: "testing fd output",
	}
	if err := reporter.Emit(ev); err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	_ = reporter.Close()

	decoder := progress.NewDecoder(r)
	var received progress.Event
	if err := decoder.Decode(&received); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if received.Message != "testing fd output" {
		t.Errorf("expected message 'testing fd output', got %q", received.Message)
	}
}

func TestReporter_NilSafe(t *testing.T) {
	var r *progress.Reporter
	if err := r.Emit(progress.Event{Type: progress.EventPhaseStart}); err != nil {
		t.Errorf("nil reporter Emit returned error: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("nil reporter Close returned error: %v", err)
	}
}

func TestNewReporter_StdoutStderr(t *testing.T) {
	ctx := context.Background()
	rStdout, err := progress.NewReporter(ctx, "stdout")
	if err != nil {
		t.Fatalf("stdout reporter failed: %v", err)
	}
	if err := rStdout.Emit(progress.Event{Type: progress.EventPhaseStart}); err != nil {
		t.Fatalf("stdout emit failed: %v", err)
	}
	_ = rStdout.Close()

	rStderr, err := progress.NewReporter(ctx, "stderr")
	if err != nil {
		t.Fatalf("stderr reporter failed: %v", err)
	}
	if err := rStderr.Emit(progress.Event{Type: progress.EventPhaseStart}); err != nil {
		t.Fatalf("stderr emit failed: %v", err)
	}
	_ = rStderr.Close()
}
