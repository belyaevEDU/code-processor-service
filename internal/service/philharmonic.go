package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/belyaevedu/remote-code-service/internal/config"
	"github.com/belyaevedu/remote-code-service/internal/domain"
	"github.com/belyaevedu/remote-code-service/internal/port"
)

// mirroring philharmonic's task.State
const (
	phrmStatePending   = 0
	phrmStateScheduled = 1
	phrmStateRunning   = 2
	phrmStateCompleted = 3
	phrmStateFailed    = 4
)

var supportedTranslators = map[string]struct{}{
	"python3": {},
	"gcc":     {},
	"clang":   {},
}

const (
	phrmDefaultPollInterval = time.Second
	phrmDefaultHTTPTimeout  = 10 * time.Second
	phrmStopTimeout         = 5 * time.Second
)

type PhilharmonicExecutor struct {
	cfg    config.PhilharmonicConfig
	client *http.Client
}

var _ port.CodeExecutor = (*PhilharmonicExecutor)(nil)

func NewPhilharmonicExecutor(cfg config.PhilharmonicConfig) *PhilharmonicExecutor {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = phrmDefaultPollInterval
	}
	if cfg.PollTimeout <= 0 {
		cfg.PollTimeout = cfg.TaskTimeout + 15*time.Second
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: phrmDefaultHTTPTimeout}
	}
	cfg.BaseURL = strings.TrimSuffix(cfg.BaseURL, "/")

	return &PhilharmonicExecutor{cfg: cfg, client: cfg.HTTPClient}
}

func (e *PhilharmonicExecutor) Execute(ctx context.Context, msg domain.TaskMessage) (domain.ExecutionResult, error) {
	if _, ok := supportedTranslators[msg.Translator]; !ok {
		return domain.ExecutionResult{}, fmt.Errorf("%w: %q", domain.ErrUnsupportedTranslator, msg.Translator)
	}
	if msg.TaskID == "" {
		return domain.ExecutionResult{}, fmt.Errorf("task message without an id")
	}

	// unique per orchestrator and traceable back to our task id
	name := "run-" + msg.TaskID

	if err := e.submit(ctx, name, msg); err != nil {
		return domain.ExecutionResult{}, fmt.Errorf("philharmonic submit: %w", err)
	}

	entry, err := e.awaitTerminal(ctx, name)
	if err != nil {
		e.stopAndLog(name)
		return domain.ExecutionResult{}, err
	}

	output, exitCode, err := e.logs(ctx, name)
	if err != nil {
		e.stopAndLog(name)
		return domain.ExecutionResult{}, fmt.Errorf("philharmonic logs: %w", err)
	}

	// a 2nd stop OR a stop on a task in a terminal state removes the record from the manager
	e.stopAndLog(name)

	result := domain.ExecutionResult{
		Output:   output,
		ExitCode: exitCode,
		Failed:   entry.State == phrmStateFailed,
	}
	if result.Failed && strings.TrimSpace(result.Output) == "" {
		if entry.FailureReason != "" {
			result.Output = fmt.Sprintf("execution failed: %s", entry.FailureReason)
		} else {
			result.Output = fmt.Sprintf("execution failed (exit code %d)", exitCode)
		}
	}
	return result, nil
}

// mirrors the philharmonic task.Task JSON field names
type submitTask struct {
	Name          string
	Image         string
	Env           []string
	RestartPolicy string
	Timeout       int64 // seconds
	Cpu           float64
	Memory        int64 // bytes
	Security      *submitSecurity
}

// mirrors the philharmonic task.Security
type submitSecurity struct {
	User            string
	CapDrop         []string
	Tmpfs           []string
	ReadOnlyRootfs  bool
	NoNewPrivileges bool
	PidsLimit       int64
	Ulimits         []submitUlimit
}

type submitUlimit struct {
	Name string `json:"Name"`
	Soft int64  `json:"Soft"`
	Hard int64  `json:"Hard"`
}

func defaultSandboxSecurity() *submitSecurity {
	return &submitSecurity{
		User:            "65532:65532",
		CapDrop:         []string{"ALL"},
		ReadOnlyRootfs:  true,
		Tmpfs:           []string{"/tmp:rw,nosuid,nodev,size=64m,mode=1777"},
		NoNewPrivileges: true,
		PidsLimit:       128,
		Ulimits: []submitUlimit{
			// fd exhaustion guard
			{Name: "nofile", Soft: 256, Hard: 256},
			// no core dumps
			{Name: "core", Soft: 0, Hard: 0},
		},
	}
}

type submitEvent struct {
	Task submitTask `json:"Task"`
}

func (e *PhilharmonicExecutor) submit(ctx context.Context, name string, msg domain.TaskMessage) error {
	body, err := json.Marshal(submitEvent{
		Task: submitTask{
			Name:  name,
			Image: e.cfg.SandboxImage,
			Env: []string{
				"TRANSLATOR=" + msg.Translator,
				"USER_CODE_B64=" + base64.StdEncoding.EncodeToString([]byte(msg.Code)),
			},
			RestartPolicy: "no",

			// ceiling-division to whole seconds
			Timeout: int64((e.cfg.TaskTimeout + time.Second - 1) / time.Second),

			Cpu:    e.cfg.Cpu,
			Memory: e.cfg.Memory,

			Security: defaultSandboxSecurity(),
		},
	})
	if err != nil {
		return err
	}

	resp, err := e.sendRequest(ctx, http.MethodPost, "/tasks", bytes.NewReader(body))
	if err != nil {
		return err
	}
	return drainAndCheck(resp, http.StatusCreated)
}

// fields of a philharmonic TaskView the client cares about
type taskListEntry struct {
	Name          string `json:"Name"`
	State         int    `json:"State"`
	FailureReason string `json:"FailureReason"`
}

// polls GET /tasks until the named task reaches a terminal state (Completed/Failed) or the poll budget runs out
func (e *PhilharmonicExecutor) awaitTerminal(ctx context.Context, name string) (taskListEntry, error) {
	deadline := time.Now().Add(e.cfg.PollTimeout)

	for {
		resp, err := e.sendRequest(ctx, http.MethodGet, "/tasks", nil)
		if err != nil {
			return taskListEntry{}, fmt.Errorf("philharmonic poll: %w", err)
		}

		entries, err := decodeTaskList(resp)
		if err != nil {
			return taskListEntry{}, fmt.Errorf("philharmonic poll: %w", err)
		}

		var match *taskListEntry
		found := 0
		for i := range entries {
			if entries[i].Name == name {
				found++
				match = &entries[i]
			}
		}

		switch {
		case found > 1:
			return taskListEntry{}, fmt.Errorf("philharmonic poll: %d tasks named %q exist", found, name)
		case found == 1 && (match.State == phrmStateCompleted || match.State == phrmStateFailed):
			return *match, nil
		}

		if err := ctx.Err(); err != nil {
			return taskListEntry{}, fmt.Errorf("philharmonic poll: %w", err)
		}

		wait := time.Until(deadline)
		if wait <= 0 {
			return taskListEntry{}, fmt.Errorf("%w: task %q did not reach a terminal state within %s",
				domain.ErrExecutionTimeout, name, e.cfg.PollTimeout)
		}
		if wait > e.cfg.PollInterval {
			wait = e.cfg.PollInterval
		}

		select {
		case <-ctx.Done():
			return taskListEntry{}, fmt.Errorf("philharmonic poll: %w", ctx.Err())
		case <-time.After(wait):
		}
	}
}

// fetches stdout+stderr logs from the manager
func (e *PhilharmonicExecutor) logs(ctx context.Context, name string) (string, int, error) {
	resp, err := e.sendRequest(ctx, http.MethodGet, "/tasks/logs/"+url.PathEscape(name), nil)
	if err != nil {
		return "", 0, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("error raised closing resp body: %v\n", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", 0, unexpectedStatus(resp)
	}

	// the orchestrator bounds the captured log size itself
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}

	exitCode := -1
	if v := resp.Header.Get("X-Exit-Code"); v != "" {
		code, err := strconv.Atoi(v)
		if err != nil {
			log.Printf("philharmonic: malformed X-Exit-Code header %q for task %s: %v", v, name, err)
		} else {
			exitCode = code
		}
	}
	return string(body), exitCode, nil
}

func (e *PhilharmonicExecutor) stopAndLog(name string) {
	if err := e.stop(name); err != nil {
		log.Printf("philharmonic: cleanup of task %q failed: %v", name, err)
	}
}

// for terminal state tasks this removes the record from the manager store,
// for live ones it stops the container
func (e *PhilharmonicExecutor) stop(name string) error {
	// fresh context: the caller's may already be expired or canceled
	ctx, cancel := context.WithTimeout(context.Background(), phrmStopTimeout)
	defer cancel()

	resp, err := e.sendRequest(ctx, http.MethodDelete, "/tasks/"+url.PathEscape(name), nil)
	if err != nil {
		return fmt.Errorf("stop request: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("error raised closing resp body: %v\n", err)
		}
	}()

	body, err := readSnippet(resp)
	if err != nil {
		return fmt.Errorf("reading stop response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusNotFound:
		return nil
	default:
		return statusError(resp.StatusCode, body)
	}
}

func (e *PhilharmonicExecutor) sendRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, method, e.cfg.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if e.cfg.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+e.cfg.Token)
	}
	return e.client.Do(httpReq)
}

func decodeTaskList(resp *http.Response) ([]taskListEntry, error) {
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("error raised closing resp body: %v\n", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, unexpectedStatus(resp)
	}

	var entries []taskListEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decoding task list: %w", err)
	}
	return entries, nil
}

func readSnippet(resp *http.Response) ([]byte, error) {
	return io.ReadAll(io.LimitReader(resp.Body, 4<<10))
}

func statusError(status int, body []byte) error {
	return fmt.Errorf("unexpected status %d: %s", status, strings.TrimSpace(string(body)))
}

// draining responses to keep the connection reusable
func drainAndCheck(resp *http.Response, want int) error {
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("error raised closing resp body: %v\n", err)
		}
	}()

	body, err := readSnippet(resp)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}
	if resp.StatusCode != want {
		return statusError(resp.StatusCode, body)
	}
	return nil
}

func unexpectedStatus(resp *http.Response) error {
	body, err := readSnippet(resp)
	if err != nil {
		return fmt.Errorf("unexpected status %d (body unreadable: %w)", resp.StatusCode, err)
	}
	return statusError(resp.StatusCode, body)
}

// wire shapes of philharmonic manager's POST /images endpoint
type pullImagesRequest struct {
	Image string `json:"image"`
}

type pullImagesReport struct {
	Image   string            `json:"image"`
	Results []pullImageResult `json:"results"`
}

type pullImageResult struct {
	Worker string `json:"worker"`
	OK     bool   `json:"ok"`
	Pulled bool   `json:"pulled"`
	Error  string `json:"error"`
}

// asks manager to pull the sandbox image on all workers
func (e *PhilharmonicExecutor) PreWarm(ctx context.Context) error {
	body, err := json.Marshal(pullImagesRequest{Image: e.cfg.SandboxImage})
	if err != nil {
		return err
	}

	resp, err := e.sendRequest(ctx, http.MethodPost, "/images", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("error raised closing resp body: %v\n", err)
		}
	}()

	// the manager always answers 200 and reports the pull per worker
	if resp.StatusCode != http.StatusOK {
		snippet, err := readSnippet(resp)
		if err != nil {
			return fmt.Errorf("reading pre-warm response: %w", err)
		}
		return statusError(resp.StatusCode, snippet)
	}

	var report pullImagesReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return fmt.Errorf("decoding pull report: %w", err)
	}

	for _, res := range report.Results {
		if !res.OK {
			return fmt.Errorf("worker %s: %s", res.Worker, res.Error)
		}
	}
	return nil
}
