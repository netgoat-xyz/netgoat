package anomaly

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

type LocalSettings struct {
	Enabled      bool
	Threshold    float64
	ModelPath    string 
	ScalerPath   string 
	PythonScript string 
}

type LocalDetector struct {
	settings LocalSettings
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   *bufio.Reader
	mu       sync.Mutex
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

	// Start Python subprocess
	cmd := exec.Command("python3", s.PythonScript, s.ModelPath, s.ScalerPath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout: %w", err)
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start python server: %w", err)
	}

	d := &LocalDetector{
		settings: s,
		cmd:      cmd,
		stdin:    stdin,
		stdout:   bufio.NewReader(stdout),
	}

	return d, nil
}

func (d *LocalDetector) PredictCSV(ctx context.Context, csv string) (label string, score float64, err error) {
	if !d.settings.Enabled {
		return "", 0, errors.New("local detector disabled")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	done := make(chan error, 1)
	var result struct {
		Label      string  `json:"label"`
		Score      float64 `json:"score"`
		Confidence float64 `json:"confidence"`
		Error      string  `json:"error"`
	}

	go func() {
		// Send CSV to Python subprocess
		if _, err := fmt.Fprintln(d.stdin, strings.TrimSpace(csv)); err != nil {
			done <- err
			return
		}

		// Read response
		line, err := d.stdout.ReadString('\n')
		if err != nil {
			done <- err
			return
		}

		if err := json.Unmarshal([]byte(line), &result); err != nil {
			done <- err
			return
		}

		done <- nil
	}()

	select {
	case <-ctx.Done():
		return "", 0, ctx.Err()
	case err := <-done:
		if err != nil {
			return "", 0, err
		}
		if result.Error != "" {
			return "", 0, errors.New(result.Error)
		}
		return result.Label, result.Score, nil
	case <-time.After(5 * time.Second):
		return "", 0, errors.New("prediction timeout")
	}
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
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stdin != nil {
		d.stdin.Close()
	}
	if d.cmd != nil && d.cmd.Process != nil {
		d.cmd.Process.Kill()
	}
	return nil
}
