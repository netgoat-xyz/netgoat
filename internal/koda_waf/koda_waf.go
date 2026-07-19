package koda_waf

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
	ModelPath    string // Path to smart_waf_model.pkl
	ScalerPath   string // Path to model_features.pkl (feature column names)
	PythonScript string
	Timeout      time.Duration
}

type Detector struct {
	settings Settings
	worker   *aiworker.Worker
}

func NewDetector(s Settings) (*Detector, error) {
	if !s.Enabled {
		return nil, errors.New("koda-waf disabled")
	}
	if _, err := os.Stat(s.ModelPath); err != nil {
		return nil, fmt.Errorf("koda-waf model file not found: %w", err)
	}
	if _, err := os.Stat(s.ScalerPath); err != nil {
		return nil, fmt.Errorf("koda-waf scaler file not found: %w", err)
	}
	if _, err := os.Stat(s.PythonScript); err != nil {
		return nil, fmt.Errorf("koda-waf python script not found: %w", err)
	}

	worker, err := aiworker.New(aiworker.Config{
		Name:            "koda-waf worker",
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
	Label      string  `json:"label"`
	Score      float64 `json:"score"`
	Confidence float64 `json:"confidence"`
	AttackType string  `json:"attack_type,omitempty"`
	Error      string  `json:"error,omitempty"`
}

func (d *Detector) Predict(ctx context.Context, csv string) (*Prediction, error) {
	if !d.settings.Enabled {
		return nil, errors.New("koda-waf disabled")
	}
	csv = strings.TrimSpace(csv)
	if len(csv) > maxFeatureInputBytes {
		return nil, fmt.Errorf("koda-waf feature input too large: %d bytes", len(csv))
	}
	if csv == "" {
		return nil, errors.New("koda-waf feature input is empty")
	}

	result := &Prediction{}
	if err := d.worker.Request(ctx, csv, result); err != nil {
		if errors.Is(err, aiworker.ErrRequestTimeout) {
			return nil, errors.New("koda-waf prediction timeout")
		}
		return nil, err
	}
	if result.Error != "" {
		return nil, errors.New(result.Error)
	}
	return result, nil
}

func (d *Detector) IsBlocked(p *Prediction) bool {
	if p == nil {
		return false
	}
	if p.Score >= d.settings.Threshold {
		return true
	}
	lab := strings.ToLower(p.Label)
	if strings.Contains(lab, "sqli") || strings.Contains(lab, "xss") || strings.Contains(lab, "rfi") || strings.Contains(lab, "lfi") || strings.Contains(lab, "command") || strings.Contains(lab, "attack") {
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
