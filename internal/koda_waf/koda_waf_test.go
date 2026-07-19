package koda_waf

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestDetectorUsesManagedWorker(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is required for detector integration tests")
	}
	model := filepath.Join(t.TempDir(), "model")
	scaler := filepath.Join(t.TempDir(), "scaler")
	for _, path := range []string{model, scaler} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatalf("create fixture %s: %v", path, err)
		}
	}

	detector, err := NewDetector(Settings{
		Enabled:      true,
		Threshold:    0.7,
		ModelPath:    model,
		ScalerPath:   scaler,
		PythonScript: filepath.Join("..", "aiworker", "testdata", "fake_worker.py"),
		Timeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("NewDetector() error = %v", err)
	}
	t.Cleanup(func() {
		if err := detector.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	prediction, err := detector.Predict(context.Background(), "1,2,3,4,5,6")
	if err != nil {
		t.Fatalf("Predict() error = %v", err)
	}
	if prediction.Label != "1,2,3,4,5,6" || prediction.Score != 0.75 {
		t.Fatalf("Predict() = %+v", prediction)
	}
	if !detector.IsBlocked(prediction) {
		t.Fatal("IsBlocked() = false, want true")
	}
}
