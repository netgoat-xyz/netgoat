// Package aiworker manages a serialized, line-delimited JSON subprocess.
package aiworker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"
)

const (
	defaultRequestTimeout   = 5 * time.Second
	defaultMaxRequestBytes  = 8 << 10
	defaultMaxResponseBytes = 64 << 10
)

var (
	ErrClosed            = errors.New("AI worker is closed")
	ErrRequestTimeout    = errors.New("AI worker request timed out")
	ErrResponseTooLarge  = errors.New("AI worker response is too large")
	ErrRequestHasNewline = errors.New("AI worker request contains a newline")
)

type Config struct {
	Name             string
	PythonExecutable string
	PythonScript     string
	Args             []string
	RequestTimeout   time.Duration
	MaxRequestBytes  int
	MaxResponseBytes int
	Stderr           io.Writer
}

// Worker owns one Python process. A token channel serializes requests while
// allowing callers waiting behind an inference to honor their own contexts.
type Worker struct {
	config Config
	token  chan struct{}

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	closed bool
}

func New(config Config) (*Worker, error) {
	config = withDefaults(config)
	if strings.TrimSpace(config.PythonScript) == "" {
		return nil, errors.New("AI worker Python script is required")
	}
	config.Args = append([]string(nil), config.Args...)
	worker := &Worker{
		config: config,
		token:  make(chan struct{}, 1),
	}
	worker.token <- struct{}{}

	if err := worker.acquire(context.Background()); err != nil {
		return nil, err
	}
	defer worker.release()
	if err := worker.startLocked(); err != nil {
		return nil, err
	}
	return worker, nil
}

func withDefaults(config Config) Config {
	if config.Name == "" {
		config.Name = "AI worker"
	}
	if config.PythonExecutable == "" {
		config.PythonExecutable = "python3"
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = defaultRequestTimeout
	}
	if config.MaxRequestBytes <= 0 {
		config.MaxRequestBytes = defaultMaxRequestBytes
	}
	if config.MaxResponseBytes <= 0 {
		config.MaxResponseBytes = defaultMaxResponseBytes
	}
	if config.Stderr == nil {
		config.Stderr = os.Stderr
	}
	return config
}

func (w *Worker) Request(ctx context.Context, request string, response any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateResponse(response); err != nil {
		return fmt.Errorf("%s response target: %w", w.config.Name, err)
	}
	request = strings.TrimSpace(request)
	if request == "" {
		return fmt.Errorf("%s request is empty", w.config.Name)
	}
	if len(request) > w.config.MaxRequestBytes {
		return fmt.Errorf("%s request is too large: %d bytes (maximum %d)", w.config.Name, len(request), w.config.MaxRequestBytes)
	}
	if strings.ContainsAny(request, "\r\n") {
		return fmt.Errorf("%s: %w", w.config.Name, ErrRequestHasNewline)
	}

	requestCtx, cancel := context.WithTimeout(ctx, w.config.RequestTimeout)
	defer cancel()
	if err := w.acquire(requestCtx); err != nil {
		return w.contextError(ctx, err)
	}
	defer w.release()
	if err := requestCtx.Err(); err != nil {
		return w.contextError(ctx, err)
	}

	if w.closed {
		return fmt.Errorf("%s: %w", w.config.Name, ErrClosed)
	}
	if w.cmd == nil {
		if err := w.startLocked(); err != nil {
			return err
		}
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		lastErr = w.requestOnceLocked(requestCtx, request, response)
		if lastErr == nil {
			return nil
		}
		if requestCtx.Err() != nil {
			return w.contextError(ctx, lastErr)
		}
		if err := w.restartLocked(); err != nil {
			return fmt.Errorf("%s request failed: %w; restart failed: %v", w.config.Name, lastErr, err)
		}
	}
	return fmt.Errorf("%s request failed after retry: %w", w.config.Name, lastErr)
}

func validateResponse(response any) error {
	if response == nil {
		return nil
	}
	destination := reflect.ValueOf(response)
	if destination.Kind() != reflect.Pointer || destination.IsNil() {
		return &json.InvalidUnmarshalError{Type: destination.Type()}
	}
	return nil
}

type readResult struct {
	line []byte
	err  error
}

func (w *Worker) requestOnceLocked(ctx context.Context, request string, response any) error {
	if _, err := fmt.Fprintln(w.stdin, request); err != nil {
		return fmt.Errorf("write request: %w", err)
	}

	result := make(chan readResult, 1)
	reader := w.stdout
	go func() {
		line, err := readLine(reader, w.config.MaxResponseBytes)
		result <- readResult{line: line, err: err}
	}()

	select {
	case <-ctx.Done():
		stopErr := w.stopLocked()
		// Killing and waiting for the old process closes its stdout. Wait for
		// that exact reader to exit before a replacement can be started.
		<-result
		if restartErr := w.startLocked(); restartErr != nil {
			return fmt.Errorf("%w; restart failed: %v", ctx.Err(), restartErr)
		}
		if stopErr != nil {
			return fmt.Errorf("%w; stopping timed-out worker: %v", ctx.Err(), stopErr)
		}
		return ctx.Err()
	case result := <-result:
		if result.err != nil {
			return fmt.Errorf("read response: %w", result.err)
		}
		if err := decodeResponse(result.line, response); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		return nil
	}
}

func decodeResponse(data []byte, response any) error {
	if response == nil {
		var discarded any
		return json.Unmarshal(data, &discarded)
	}

	destination := reflect.ValueOf(response)
	// Decode into a fresh value so a malformed first response cannot leave
	// partially populated state behind when the request is retried.
	decoded := reflect.New(destination.Elem().Type())
	if err := json.Unmarshal(data, decoded.Interface()); err != nil {
		return err
	}
	destination.Elem().Set(decoded.Elem())
	return nil
}

func readLine(reader *bufio.Reader, limit int) ([]byte, error) {
	line := make([]byte, 0, min(limit, 4096))
	for {
		fragment, err := reader.ReadSlice('\n')
		if len(line)+len(fragment) > limit+1 {
			return nil, fmt.Errorf("%w (maximum %d bytes)", ErrResponseTooLarge, limit)
		}
		line = append(line, fragment...)
		switch {
		case err == nil:
			line = bytesTrimLineEnding(line)
			if len(line) > limit {
				return nil, fmt.Errorf("%w (maximum %d bytes)", ErrResponseTooLarge, limit)
			}
			return line, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		default:
			return nil, err
		}
	}
}

func bytesTrimLineEnding(line []byte) []byte {
	line = line[:len(line)-1]
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	return line
}

func (w *Worker) contextError(parent context.Context, err error) error {
	if parentErr := parent.Err(); parentErr != nil {
		return parentErr
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w", w.config.Name, ErrRequestTimeout)
	}
	return err
}

func (w *Worker) acquire(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-w.token:
		return nil
	}
}

func (w *Worker) release() {
	w.token <- struct{}{}
}

func (w *Worker) startLocked() error {
	args := make([]string, 0, len(w.config.Args)+2)
	args = append(args, "-u", w.config.PythonScript)
	args = append(args, w.config.Args...)
	cmd := exec.Command(w.config.PythonExecutable, args...)
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")
	cmd.Stderr = w.config.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("%s failed to create stdin: %w", w.config.Name, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("%s failed to create stdout: %w", w.config.Name, err)
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return fmt.Errorf("%s failed to start: %w", w.config.Name, err)
	}

	w.cmd = cmd
	w.stdin = stdin
	w.stdout = bufio.NewReader(stdout)
	return nil
}

func (w *Worker) restartLocked() error {
	stopErr := w.stopLocked()
	startErr := w.startLocked()
	if startErr != nil {
		return startErr
	}
	return stopErr
}

func (w *Worker) stopLocked() error {
	cmd := w.cmd
	stdin := w.stdin
	w.cmd = nil
	w.stdin = nil
	w.stdout = nil

	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd == nil {
		return nil
	}
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	err := cmd.Wait()
	var exitErr *exec.ExitError
	if err == nil || errors.As(err, &exitErr) || errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}

func (w *Worker) Close() error {
	if err := w.acquire(context.Background()); err != nil {
		return err
	}
	defer w.release()
	if w.closed {
		return nil
	}
	w.closed = true
	return w.stopLocked()
}
