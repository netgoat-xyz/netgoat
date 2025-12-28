package anomaly

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

// Settings carries the minimal configuration needed by the detector.
type Settings struct {
	Enabled   bool
	Threshold float64
	Model     string
	Token     string
}

type Detector struct {
	settings Settings
	client   *http.Client
	endpoint string
}

func NewDetector(s Settings) *Detector {
	if s.Model == "" {
		s.Model = "netgoat-ai/GoatAI"
	}
	if s.Token == "" {
		// Try common env var names
		if v := os.Getenv("HUGGINGFACE_TOKEN"); v != "" {
			s.Token = v
		} else if v := os.Getenv("HUGGINGFACEHUB_API_TOKEN"); v != "" {
			s.Token = v
		}
	}
	return &Detector{
		settings: s,
		client:   &http.Client{Timeout: 10 * time.Second},
		endpoint: "https://api-inference.huggingface.co/models/" + s.Model,
	}
}

// PredictCSV sends the CSV feature vector to the model and returns the best (label, score).
// CSV must be in the expected order: Flow Duration, Total Fwd Packets, Total Backward Packets,
// Packet Length Mean, Flow IAT Mean, Fwd Flag Count
func (d *Detector) PredictCSV(ctx context.Context, csv string) (label string, score float64, err error) {
	if !d.settings.Enabled {
		return "", 0, errors.New("anomaly detector disabled")
	}
	payload := map[string]any{
		"inputs":  csv,
		"options": map[string]any{"wait_for_model": true},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", d.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if d.settings.Token != "" {
		req.Header.Set("Authorization", "Bearer "+d.settings.Token)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", 0, errors.New("inference API returned status: " + resp.Status)
	}
	var raw any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return "", 0, err
	}
	return parseHFClassification(raw)
}

// parseHFClassification supports common HF responses: either an array of {label,score}
// or a nested single-element array [[{label,score},...]].
func parseHFClassification(raw any) (string, float64, error) {
	slice, ok := raw.([]any)
	if !ok {
		// try nested
		outer, ok := raw.([][]map[string]any)
		if ok && len(outer) > 0 {
			return bestFromMaps(outer[0])
		}
		// last attempt: []map[string]any but typed as []any
		if s2, ok := raw.([][]any); ok && len(s2) > 0 {
			return bestFromAnySlice(s2[0])
		}
		return "", 0, errors.New("unexpected HF response shape")
	}
	return bestFromAnySlice(slice)
}

func bestFromAnySlice(items []any) (string, float64, error) {
	maps := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if m, ok := it.(map[string]any); ok {
			maps = append(maps, m)
		}
	}
	return bestFromMaps(maps)
}

func bestFromMaps(items []map[string]any) (string, float64, error) {
	topLabel := ""
	topScore := -1.0
	for _, m := range items {
		lab, _ := m["label"].(string)
		sc, _ := m["score"].(float64)
		if sc > topScore {
			topScore = sc
			topLabel = lab
		}
	}
	if topLabel == "" {
		return "", 0, errors.New("no predictions in response")
	}
	return topLabel, topScore, nil
}

// IsAnomalous decides whether to block based on (label,score) and the configured threshold.
func (d *Detector) IsAnomalous(label string, score float64) bool {
	if score >= d.settings.Threshold {
		return true
	}
	// fallback: treat clearly anomalous labels as block-worthy even if score slightly lower
	lab := strings.ToLower(label)
	if strings.Contains(lab, "anom") || strings.Contains(lab, "malicious") || strings.Contains(lab, "attack") {
		return score >= max(0.5, d.settings.Threshold*0.8)
	}
	return false
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
