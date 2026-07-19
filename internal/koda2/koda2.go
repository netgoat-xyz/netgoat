package koda2

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

type Settings struct {
	Enabled      bool
	Threshold    float64
	ModelPath    string
	ScalerPath   string
	PythonScript string
	Timeout      time.Duration
}

type Detector struct {
	settings Settings
	worker   *aiworker.Worker
}

func NewDetector(s Settings) (*Detector, error) {
	if !s.Enabled {
		return nil, errors.New("koda-2 disabled")
	}
	if _, err := os.Stat(s.ModelPath); err != nil {
		return nil, fmt.Errorf("koda-2 model file not found: %w", err)
	}
	if _, err := os.Stat(s.ScalerPath); err != nil {
		return nil, fmt.Errorf("koda-2 scaler file not found: %w", err)
	}
	if _, err := os.Stat(s.PythonScript); err != nil {
		return nil, fmt.Errorf("koda-2 python script not found: %w", err)
	}

	worker, err := aiworker.New(aiworker.Config{
		Name:            "koda-2 worker",
		PythonScript:    s.PythonScript,
		Args:            []string{s.ModelPath, s.ScalerPath},
		RequestTimeout:  s.Timeout,
		MaxRequestBytes: maxFeatureInputBytes,
		Stderr:          os.Stderr,
	})
	if err != nil {
		return nil, err
	}
	return &Detector{settings: s, worker: worker}, nil
}

type Prediction struct {
	Label           string             `json:"label"`
	Score           float64            `json:"score"`
	Confidence      float64            `json:"confidence"`
	VectorBreakdown map[string]float64 `json:"vector_breakdown,omitempty"`
	Error           string             `json:"error,omitempty"`
}

func (d *Detector) Predict(ctx context.Context, csv string) (*Prediction, error) {
	if !d.settings.Enabled {
		return nil, errors.New("koda-2 disabled")
	}
	csv = strings.TrimSpace(csv)
	if len(csv) > maxFeatureInputBytes {
		return nil, fmt.Errorf("koda-2 feature input too large: %d bytes", len(csv))
	}
	if csv == "" {
		return nil, errors.New("koda-2 feature input is empty")
	}

	result := &Prediction{}
	if err := d.worker.Request(ctx, csv, result); err != nil {
		if errors.Is(err, aiworker.ErrRequestTimeout) {
			return nil, errors.New("koda-2 prediction timeout")
		}
		return nil, err
	}
	if result.Error != "" {
		return nil, errors.New(result.Error)
	}
	return result, nil
}

func (d *Detector) IsAnomalous(p *Prediction) bool {
	if p == nil {
		return false
	}
	if p.Score >= d.settings.Threshold {
		return true
	}
	lab := strings.ToLower(p.Label)
	if strings.Contains(lab, "anom") || strings.Contains(lab, "malicious") || strings.Contains(lab, "attack") || strings.Contains(lab, "threat") {
		return p.Score >= d.settings.Threshold*0.8
	}
	return false
}

func (d *Detector) Close() error {
	if d.worker == nil {
		return nil
	}
	return d.worker.Close()
}
