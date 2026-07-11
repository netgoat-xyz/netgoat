package koda2

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
	ModelPath    string
	ScalerPath   string
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
		return fmt.Errorf("koda-2 failed to create stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("koda-2 failed to create stdout: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("koda-2 failed to start python server: %w", err)
	}

	d.cmd = cmd
	d.stdin = stdin
	d.stdout = bufio.NewReader(stdout)
	return nil
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
			return nil, fmt.Errorf("koda-2 failed to write request: %w; restart failed: %v", err, restartErr)
		}
		return nil, err
	}

	go func() {
		line, err := d.stdout.ReadString('\n')
		if err != nil {
			done <- err
			return
		}
		if err := json.Unmarshal([]byte(line), result); err != nil {
			done <- err
			return
		}
		done <- nil
	}()

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		if restartErr := d.restartLocked(); restartErr != nil {
			return nil, fmt.Errorf("%w; koda-2 restart failed: %v", ctx.Err(), restartErr)
		}
		return nil, ctx.Err()
	case err := <-done:
		if err != nil {
			if restartErr := d.restartLocked(); restartErr != nil {
				return nil, fmt.Errorf("koda-2 prediction failed: %w; restart failed: %v", err, restartErr)
			}
			return nil, err
		}
		if result.Error != "" {
			return nil, errors.New(result.Error)
		}
		return result, nil
	case <-timer.C:
		if restartErr := d.restartLocked(); restartErr != nil {
			return nil, fmt.Errorf("koda-2 prediction timeout; restart failed: %w", restartErr)
		}
		return nil, errors.New("koda-2 prediction timeout")
	}
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
