package aiworker

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testResponse struct {
	Label string  `json:"label"`
	Score float64 `json:"score"`
}

func newTestWorker(t *testing.T, configure func(*Config)) *Worker {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required for AI worker integration tests")
	}

	config := Config{
		Name:             "test worker",
		PythonExecutable: python,
		PythonScript:     filepath.Join("testdata", "fake_worker.py"),
		RequestTimeout:   time.Second,
		MaxRequestBytes:  8192,
		MaxResponseBytes: 8192,
		Stderr:           io.Discard,
	}
	if configure != nil {
		configure(&config)
	}
	worker, err := New(config)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() {
		if err := worker.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return worker
}

func TestWorkerRunsPythonUnbuffered(t *testing.T) {
	worker := newTestWorker(t, func(config *Config) {
		config.RequestTimeout = 500 * time.Millisecond
	})

	var response testResponse
	if err := worker.Request(context.Background(), "unbuffered", &response); err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	if response.Label != "unbuffered" || response.Score != 0.75 {
		t.Fatalf("Request() response = %+v", response)
	}
}

func TestWorkerRestartsAfterTimeoutWithoutStaleRead(t *testing.T) {
	worker := newTestWorker(t, func(config *Config) {
		config.RequestTimeout = 100 * time.Millisecond
	})

	var response testResponse
	err := worker.Request(context.Background(), "hang", &response)
	if !errors.Is(err, ErrRequestTimeout) {
		t.Fatalf("Request(hang) error = %v, want ErrRequestTimeout", err)
	}

	if err := worker.Request(context.Background(), "after-timeout", &response); err != nil {
		t.Fatalf("Request(after-timeout) error = %v", err)
	}
	if response.Label != "after-timeout" {
		t.Fatalf("Request(after-timeout) label = %q", response.Label)
	}
}

func TestWorkerRestartsAndRetriesBrokenProcesses(t *testing.T) {
	worker := newTestWorker(t, nil)

	t.Run("process exit", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "crashed")
		var response testResponse
		if err := worker.Request(context.Background(), "crash-once:"+marker, &response); err != nil {
			t.Fatalf("Request() error = %v", err)
		}
		if response.Label != "restarted" {
			t.Fatalf("Request() label = %q, want restarted", response.Label)
		}
	})

	t.Run("invalid response type", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "invalid")
		response := testResponse{Label: "original", Score: 1}
		if err := worker.Request(context.Background(), "bad-type-once:"+marker, &response); err != nil {
			t.Fatalf("Request() error = %v", err)
		}
		if response.Label != "" || response.Score != 0.75 {
			t.Fatalf("Request() response = %+v; stale fields survived retry", response)
		}
	})
}

func TestWorkerBoundsProtocolMessagesAndRecovers(t *testing.T) {
	worker := newTestWorker(t, func(config *Config) {
		config.MaxRequestBytes = 16
		config.MaxResponseBytes = 128
	})

	var response testResponse
	if err := worker.Request(context.Background(), strings.Repeat("x", 17), &response); err == nil {
		t.Fatal("Request(oversized) error = nil")
	}
	if err := worker.Request(context.Background(), "first\nsecond", &response); !errors.Is(err, ErrRequestHasNewline) {
		t.Fatalf("Request(multiline) error = %v, want ErrRequestHasNewline", err)
	}

	err := worker.Request(context.Background(), "large", &response)
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("Request(large) error = %v, want ErrResponseTooLarge", err)
	}
	if err := worker.Request(context.Background(), "recovered", &response); err != nil {
		t.Fatalf("Request(recovered) error = %v", err)
	}
	if response.Label != "recovered" {
		t.Fatalf("Request(recovered) label = %q", response.Label)
	}
}

func TestQueuedRequestHonorsCallerCancellation(t *testing.T) {
	worker := newTestWorker(t, func(config *Config) {
		config.RequestTimeout = 2 * time.Second
	})
	marker := filepath.Join(t.TempDir(), "started")

	firstContext, cancelFirst := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancelFirst()
	firstDone := make(chan error, 1)
	go func() {
		var response testResponse
		firstDone <- worker.Request(firstContext, "hang-ready:"+marker, &response)
	}()

	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stat marker: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatal("first request did not reach worker")
		}
		time.Sleep(5 * time.Millisecond)
	}

	queuedContext, cancelQueued := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelQueued()
	var response testResponse
	started := time.Now()
	err := worker.Request(queuedContext, "queued", &response)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("queued Request() error = %v, want context deadline", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("queued Request() took %v after cancellation", elapsed)
	}

	if err := <-firstDone; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("first Request() error = %v, want context deadline", err)
	}
	if err := worker.Request(context.Background(), "after-queue", &response); err != nil {
		t.Fatalf("Request(after-queue) error = %v", err)
	}
}

func TestClosedWorkerRejectsRequests(t *testing.T) {
	worker := newTestWorker(t, nil)
	if err := worker.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var response testResponse
	err := worker.Request(context.Background(), "closed", &response)
	if !errors.Is(err, ErrClosed) {
		t.Fatalf("Request() error = %v, want ErrClosed", err)
	}
}
