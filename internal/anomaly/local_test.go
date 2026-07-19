package anomaly

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalDetectorUsesManagedWorker(t *testing.T) {
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

	detector, err := NewLocalDetector(LocalSettings{
		Enabled:      true,
		Threshold:    0.7,
		ModelPath:    model,
		ScalerPath:   scaler,
		PythonScript: filepath.Join("..", "aiworker", "testdata", "fake_worker.py"),
		Timeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("NewLocalDetector() error = %v", err)
	}
	t.Cleanup(func() {
		if err := detector.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	label, score, err := detector.PredictCSV(context.Background(), "1,2,3,4,5,6")
	if err != nil {
		t.Fatalf("PredictCSV() error = %v", err)
	}
	if label != "1,2,3,4,5,6" || score != 0.75 {
		t.Fatalf("PredictCSV() = (%q, %v)", label, score)
	}
}
