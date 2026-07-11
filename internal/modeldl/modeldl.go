package modeldl

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
)

const maxModelDownloadBytes = 512 << 20

type ModelFile struct {
	URL      string
	DestPath string
	Label    string
}

func EnsureDownloaded(files []ModelFile) {
	for _, f := range files {
		if err := downloadIfMissing(f); err != nil {
			log.Warn().Err(err).Str("model", f.Label).Str("dest", f.DestPath).Msg("Model download failed, will use cached/default")
		}
	}
}

func downloadIfMissing(f ModelFile) error {
	// Check if file already exists
	if st, err := os.Stat(f.DestPath); err == nil {
		if st.IsDir() {
			return fmt.Errorf("model destination is a directory: %s", f.DestPath)
		}
		log.Debug().Str("model", f.Label).Str("path", f.DestPath).Msg("Model file already exists, skipping download")
		return nil
	}

	// Ensure parent directory exists
	dir := filepath.Dir(f.DestPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	log.Info().Str("model", f.Label).Str("url", f.URL).Str("dest", f.DestPath).Msg("Downloading model file")

	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest("GET", f.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "NetGoat/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	if resp.ContentLength > maxModelDownloadBytes {
		return fmt.Errorf("download too large: %d bytes", resp.ContentLength)
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(f.DestPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	written, err := io.Copy(tmp, io.LimitReader(resp.Body, maxModelDownloadBytes+1))
	if err != nil {
		tmp.Close()
		return fmt.Errorf("download interrupted: %w", err)
	}
	if written > maxModelDownloadBytes {
		tmp.Close()
		return fmt.Errorf("download exceeded %d bytes", maxModelDownloadBytes)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, f.DestPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	log.Info().Str("model", f.Label).Int64("bytes", written).Msg("Model file downloaded successfully")
	return nil
}
