package anomaly

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"netgoat.xyz/agent/internal/aiworker"
)

const maxFeatureInputBytes = 8192

type LocalSettings struct {
	Enabled      bool
	Threshold    float64
	ModelPath    string
	ScalerPath   string
	PythonScript string
	Timeout      time.Duration
}

type LocalDetector struct {
	settings LocalSettings
	worker   *aiworker.Worker
}

func NewLocalDetector(s LocalSettings) (*LocalDetector, error) {
	if !s.Enabled {
		return nil, errors.New("local detector disabled")
	}

	if _, err := os.Stat(s.ModelPath); err != nil {
		return nil, fmt.Errorf("model file not found: %w", err)
	}
	if _, err := os.Stat(s.ScalerPath); err != nil {
		return nil, fmt.Errorf("scaler file not found: %w", err)
	}
	if _, err := os.Stat(s.PythonScript); err != nil {
		return nil, fmt.Errorf("python script not found: %w", err)
	}

	worker, err := aiworker.New(aiworker.Config{
		Name:            "local anomaly worker",
		PythonScript:    s.PythonScript,
		Args:            []string{s.ModelPath, s.ScalerPath},
		RequestTimeout:  s.Timeout,
		MaxRequestBytes: maxFeatureInputBytes,
		Stderr:          os.Stderr,
	})
	if err != nil {
		return nil, err
	}
	return &LocalDetector{settings: s, worker: worker}, nil
}

func (d *LocalDetector) PredictCSV(ctx context.Context, csv string) (label string, score float64, err error) {
	if !d.settings.Enabled {
		return "", 0, errors.New("local detector disabled")
	}

	csv = strings.TrimSpace(csv)
	if csv == "" {
		return "", 0, errors.New("feature input is empty")
	}
	if len(csv) > maxFeatureInputBytes {
		return "", 0, fmt.Errorf("feature input too large: %d bytes", len(csv))
	}
	var result struct {
		Label      string  `json:"label"`
		Score      float64 `json:"score"`
		Confidence float64 `json:"confidence"`
		Error      string  `json:"error"`
	}
	if err := d.worker.Request(ctx, csv, &result); err != nil {
		if errors.Is(err, aiworker.ErrRequestTimeout) {
			return "", 0, errors.New("prediction timeout")
		}
		return "", 0, err
	}
	if result.Error != "" {
		return "", 0, errors.New(result.Error)
	}
	return result.Label, result.Score, nil
}

func (d *LocalDetector) IsAnomalous(label string, score float64) bool {
	if score >= d.settings.Threshold {
		return true
	}
	lab := strings.ToLower(label)
	if strings.Contains(lab, "anom") || strings.Contains(lab, "malicious") || strings.Contains(lab, "attack") {
		return score >= d.settings.Threshold*0.8
	}
	return false
}

func (d *LocalDetector) Close() error {
	if d.worker == nil {
		return nil
	}
	return d.worker.Close()
}
