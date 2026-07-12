package koda_waf

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const maxFeatureInputBytes = 8192

type Settings struct {
	Enabled      bool
	Threshold    float64
	ModelPath    string // Path to smart_waf_model.pkl
	ScalerPath   string // Path to model_features.pkl (feature column names)
	PythonScript string
}

type Detector struct {
	settings Settings
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   *bufio.Reader
	mu       sync.Mutex
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

	d := &Detector{settings: s}
	if err := d.startLocked(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Detector) startLocked() error {
	s := d.settings
	cmd := exec.Command("python3", s.PythonScript, s.ModelPath, s.ScalerPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("koda-waf failed to create stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("koda-waf failed to create stdout: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("koda-waf failed to start python server: %w", err)
	}

	d.cmd = cmd
	d.stdin = stdin
	d.stdout = bufio.NewReader(stdout)
	return nil
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

	d.mu.Lock()
	defer d.mu.Unlock()

	done := make(chan error, 1)
	result := &Prediction{}

	if d.cmd == nil || d.stdin == nil || d.stdout == nil {
		if err := d.startLocked(); err != nil {
			return nil, err
		}
	}
	if _, err := fmt.Fprintln(d.stdin, csv); err != nil {
		if restartErr := d.restartLocked(); restartErr != nil {
			return nil, fmt.Errorf("koda-waf failed to write request: %w; restart failed: %v", err, restartErr)
		}
		return nil, err
	}

	reader := d.stdout
	go func(reader *bufio.Reader) {
		line, err := reader.ReadString('\n')
		if err != nil {
			done <- err
			return
		}
		if err := json.Unmarshal([]byte(line), result); err != nil {
			done <- err
			return
		}
		done <- nil
	}(reader)

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		if restartErr := d.restartLocked(); restartErr != nil {
			return nil, fmt.Errorf("%w; koda-waf restart failed: %v", ctx.Err(), restartErr)
		}
		return nil, ctx.Err()
	case err := <-done:
		if err != nil {
			if restartErr := d.restartLocked(); restartErr != nil {
				return nil, fmt.Errorf("koda-waf prediction failed: %w; restart failed: %v", err, restartErr)
			}
			return nil, err
		}
		if result.Error != "" {
			return nil, errors.New(result.Error)
		}
		return result, nil
	case <-timer.C:
		if restartErr := d.restartLocked(); restartErr != nil {
			return nil, fmt.Errorf("koda-waf prediction timeout; restart failed: %w", restartErr)
		}
		return nil, errors.New("koda-waf prediction timeout")
	}
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
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.closeProcessLocked()
}

func (d *Detector) restartLocked() error {
	_ = d.closeProcessLocked()
	return d.startLocked()
}

func (d *Detector) closeProcessLocked() error {
	if d.stdin != nil {
		_ = d.stdin.Close()
		d.stdin = nil
	}
	if d.cmd != nil && d.cmd.Process != nil {
		_ = d.cmd.Process.Kill()
		_ = d.cmd.Wait()
	}
	d.cmd = nil
	d.stdout = nil
	return nil
}
