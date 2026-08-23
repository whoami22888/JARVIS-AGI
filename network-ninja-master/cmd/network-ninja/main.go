package main

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/whoami22888/JARVIS-AGI/network-ninja-master/internal/agent"
	"github.com/whoami22888/JARVIS-AGI/network-ninja-master/internal/audit"
	"github.com/whoami22888/JARVIS-AGI/network-ninja-master/internal/device"
	"github.com/whoami22888/JARVIS-AGI/network-ninja-master/internal/scan"
	"github.com/whoami22888/JARVIS-AGI/network-ninja-master/internal/scope"
	"github.com/whoami22888/JARVIS-AGI/network-ninja-master/internal/store"
)

var version = "0.2.0-dev"

//go:embed web/index.html
var dashboardHTML []byte

type runtimeEnv struct {
	root       string
	audit      *audit.Store
	state      *store.Store
	agent      *agent.Service
	writeToken string
}

type operationResult struct {
	JobID          string `json:"job_id"`
	Status         string `json:"status"`
	Result         any    `json:"result,omitempty"`
	Error          string `json:"error,omitempty"`
	EvidenceID     string `json:"evidence_id,omitempty"`
	EvidenceSHA256 string `json:"evidence_sha256,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		usage(2)
		return
	}
	switch os.Args[1] {
	case "version", "--version", "-version":
		fmt.Println(version)
	case "host":
		requireSubcommand(os.Args, "host inspect", hostCommand)
	case "device":
		requireSubcommand(os.Args, "device inventory", deviceCommand)
	case "scope":
		requireSubcommand(os.Args, "scope validate --file path", scopeCommand)
	case "scan":
		requireSubcommand(os.Args, "scan tcp|tls|http", scanCommand)
	case "agent":
		requireSubcommand(os.Args, "agent status", agentCommand)
	case "serve":
		serveCommand(os.Args[2:])
	default:
		usage(2)
	}
}

func requireSubcommand(args []string, example string, handler func([]string)) {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "missing subcommand; expected %s\n", example)
		os.Exit(2)
	}
	handler(args[2:])
}

func usage(code int) {
	fmt.Fprintf(os.Stderr, `Network Ninja Master %s

Usage:
  network-ninja version
  network-ninja host inspect
  network-ninja device inventory
  network-ninja scope validate --file scope.json
  network-ninja scan tcp --scope scope.json --target host --port 443
  network-ninja scan tls --scope scope.json --target host --port 443
  network-ninja scan http --scope scope.json --url https://host/
  network-ninja agent status
  network-ninja serve [--addr 127.0.0.1:8000] [--data-root path]

Active network diagnostics require an explicit, time-bounded authorization scope.
`, version)
	if code != 0 {
		os.Exit(code)
	}
}

func hostCommand(args []string) {
	if len(args) != 1 || args[0] != "inspect" {
		usage(2)
		return
	}
	host, err := device.InspectHost()
	mustJSON(host, err)
}

func deviceCommand(args []string) {
	if len(args) != 1 || args[0] != "inventory" {
		usage(2)
		return
	}
	inventory, err := device.Inventory()
	mustJSON(map[string]any{"devices": inventory, "privileged": device.IsPrivileged()}, err)
}

func scopeCommand(args []string) {
	if len(args) < 1 || args[0] != "validate" {
		usage(2)
		return
	}
	flags := flag.NewFlagSet("scope validate", flag.ExitOnError)
	file := flags.String("file", "", "scope JSON path")
	if err := flags.Parse(args[1:]); err != nil {
		fatal(err)
	}
	if *file == "" {
		fatal(errors.New("--file is required"))
	}
	parsed, err := scope.Load(*file)
	mustJSON(map[string]any{"status": "VALID", "scope": parsed}, err)
}

func scanCommand(args []string) {
	if len(args) < 1 {
		usage(2)
		return
	}
	operation, ok := map[string]scope.Operation{"tcp": scope.OperationTCP, "tls": scope.OperationTLS, "http": scope.OperationHTTP}[args[0]]
	if !ok {
		fatal(errors.New("scan operation must be tcp, tls, or http"))
	}
	flags := flag.NewFlagSet("scan "+args[0], flag.ExitOnError)
	scopePath := flags.String("scope", "", "scope JSON path")
	target := flags.String("target", "", "hostname or IP")
	port := flags.Int("port", 0, "target port")
	rawURL := flags.String("url", "", "http(s) URL")
	dataRoot := flags.String("data-root", "", "persistent data root")
	if err := flags.Parse(args[1:]); err != nil {
		fatal(err)
	}
	if *scopePath == "" {
		fatal(errors.New("--scope is required"))
	}
	parsed, err := scope.Load(*scopePath)
	if err != nil {
		fatal(err)
	}
	if operation == scope.OperationHTTP {
		if *rawURL == "" {
			fatal(errors.New("--url is required for HTTP metadata"))
		}
		var parseErr error
		*target, *port, parseErr = scan.HostPort(*rawURL)
		if parseErr != nil {
			fatal(parseErr)
		}
	} else if *target == "" || *port == 0 {
		fatal(errors.New("--target and --port are required"))
	}
	runtime, err := openRuntime(*dataRoot)
	if err != nil {
		fatal(err)
	}
	defer runtime.state.Close()
	result := runtime.run(operation, parsed, *target, *port, *rawURL)
	mustJSON(result, nil)
	if result.Status == "BLOCKED" || result.Status == "FAILED" {
		os.Exit(1)
	}
}

func agentCommand(args []string) {
	if len(args) != 1 || args[0] != "status" {
		usage(2)
		return
	}
	endpoint := strings.TrimSpace(os.Getenv("NETWORK_NINJA_LOCAL_MODEL_ENDPOINT"))
	status := "LOCAL_MODEL_UNAVAILABLE"
	if endpoint != "" {
		status = "LOCAL_MODEL_CONFIGURED_NOT_VALIDATED"
	}
	mustJSON(map[string]any{
		"status":       status,
		"endpoint":     endpoint,
		"planner":      "no model is trusted to execute tools directly; operator-selected guarded operations remain required",
		"remote_model": "REMOTE_MODEL_NOT_CONFIGURED",
	}, nil)
}

func serveCommand(args []string) {
	flags := flag.NewFlagSet("serve", flag.ExitOnError)
	address := flags.String("addr", "127.0.0.1:8000", "loopback HTTP address")
	dataRoot := flags.String("data-root", "", "persistent data root")
	if err := flags.Parse(args); err != nil {
		fatal(err)
	}
	host, _, err := net.SplitHostPort(*address)
	if err != nil || !isLoopbackHost(host) {
		fatal(errors.New("dashboard may bind only to loopback (for example 127.0.0.1:8000)"))
	}
	runtime, err := openRuntime(*dataRoot)
	if err != nil {
		fatal(err)
	}
	defer runtime.state.Close()
	server := &http.Server{Addr: *address, Handler: runtime.routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	fmt.Printf("Network Ninja Master dashboard: http://%s\nData root: %s\n", *address, runtime.root)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fatal(err)
	}
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func openRuntime(requestedRoot string) (*runtimeEnv, error) {
	root, err := resolveDataRoot(requestedRoot)
	if err != nil {
		return nil, err
	}
	auditStore, err := audit.Open(root)
	if err != nil {
		return nil, err
	}
	state, err := store.Open(root)
	if err != nil {
		return nil, err
	}
	writeToken, err := newWriteToken()
	if err != nil {
		_ = state.Close()
		return nil, err
	}
	return &runtimeEnv{root: root, audit: auditStore, state: state, agent: agent.New(state), writeToken: writeToken}, nil
}

func newWriteToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate local write token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func resolveDataRoot(requested string) (string, error) {
	candidates := []string{requested, os.Getenv("NETWORK_NINJA_DATA_ROOT")}
	if usbRoot := os.Getenv("NETWORK_NINJA_USB_ROOT"); usbRoot != "" {
		candidates = append(candidates, filepath.Join(usbRoot, "data"))
	}
	if user, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(user, ".local", "share", "network-ninja"))
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if err := os.MkdirAll(absolute, 0o700); err == nil {
			return absolute, nil
		}
	}
	return "", errors.New("no writable data root is available")
}

func (r *runtimeEnv) run(operation scope.Operation, engagement scope.Scope, target string, port int, rawURL string) operationResult {
	now := time.Now().UTC()
	decision := engagement.Authorize(operation, target, port, now)
	if !decision.Allowed {
		_ = r.audit.Record(audit.Event{Timestamp: now, EventID: newJobID(), EngagementID: engagement.EngagementID, Operator: engagement.Operator, Operation: string(operation), Target: target, Status: "BLOCKED", Decision: decision.Reason})
		return operationResult{Status: "BLOCKED", Error: decision.Reason}
	}
	jobID := newJobID()
	if err := r.state.Start(store.Job{ID: jobID, EngagementID: engagement.EngagementID, Operation: string(operation), Target: target, Status: "IN_PROGRESS", StartedAt: now}); err != nil {
		return operationResult{JobID: jobID, Status: "FAILED", Error: "persist job start: " + err.Error()}
	}
	_ = r.audit.Record(audit.Event{Timestamp: now, EventID: jobID, EngagementID: engagement.EngagementID, Operator: engagement.Operator, Operation: string(operation), Target: target, Status: "IN_PROGRESS", Decision: decision.Reason})
	context, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var result any
	var status = "TESTED"
	var runErr string
	switch operation {
	case scope.OperationTCP:
		value := scan.TCP(context, target, port)
		result = value
		if value.Error != "" {
			status, runErr = "FAILED", value.Error
		}
	case scope.OperationTLS:
		value := scan.TLS(context, target, port)
		result = value
		if value.Error != "" {
			status, runErr = "FAILED", value.Error
		}
	case scope.OperationHTTP:
		value := scan.HTTP(context, rawURL)
		result = value
		if value.Error != "" {
			status, runErr = "FAILED", value.Error
		}
	default:
		status, runErr, result = "BLOCKED", "operation is not implemented", map[string]string{"reason": "unsupported operation"}
	}
	evidenceID, evidenceSHA, evidenceErr := r.audit.Evidence(string(operation), map[string]any{"job_id": jobID, "scope": engagement, "target": target, "port": port, "operation": operation, "result": result})
	if evidenceErr != nil {
		status, runErr = "FAILED", "write evidence: "+evidenceErr.Error()
	}
	_ = r.state.Finish(jobID, status, evidenceID, evidenceSHA, runErr, result)
	_ = r.audit.Record(audit.Event{Timestamp: time.Now().UTC(), EventID: jobID, EngagementID: engagement.EngagementID, Operator: engagement.Operator, Operation: string(operation), Target: target, Status: status, Decision: decision.Reason, EvidenceID: evidenceID, EvidenceSHA: evidenceSHA, Detail: map[string]any{"error": runErr}})
	return operationResult{JobID: jobID, Status: status, Result: result, Error: runErr, EvidenceID: evidenceID, EvidenceSHA256: evidenceSHA}
}

func newJobID() string {
	return fmt.Sprintf("job-%s", time.Now().UTC().Format("20060102T150405.000000000Z"))
}

func (r *runtimeEnv) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/" {
			http.NotFound(w, request)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(dashboardHTML)
	})
	mux.HandleFunc("/api/health", r.healthAPI)
	mux.HandleFunc("/api/host", r.hostAPI)
	mux.HandleFunc("/api/devices", r.devicesAPI)
	mux.HandleFunc("/api/jobs", r.jobsAPI)
	mux.HandleFunc("/api/scan/", r.scanAPI)
	mux.HandleFunc("/api/agent/status", r.agentStatusAPI)
	mux.HandleFunc("/api/agent/context", r.agentContextAPI)
	mux.HandleFunc("/api/agent/chat", r.agentChatAPI)
	mux.HandleFunc("/api/agent/messages", r.agentMessagesAPI)
	mux.HandleFunc("/api/agent/memories", r.agentMemoriesAPI)
	mux.HandleFunc("/api/agent/activity", r.agentActivityAPI)
	mux.HandleFunc("/api/agent/commands", r.agentCommandsAPI)
	mux.HandleFunc("/api/agent/commands/", r.agentCommandAPI)
	mux.HandleFunc("/api/agent/tasks", r.agentTasksAPI)
	mux.HandleFunc("/api/agent/tasks/", r.agentTaskAPI)
	mux.HandleFunc("/api/agent/takeover", r.agentTakeoverAPI)
	mux.HandleFunc("/api/agent/skills", r.agentSkillsAPI)
	mux.HandleFunc("/api/agent/skills/", r.agentSkillAPI)
	return securityHeaders(r.requireWriteToken(mux))
}

func (r *runtimeEnv) healthAPI(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	integrity, err := r.state.IntegrityCheck()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "version": version, "data_root": r.root, "database_integrity": integrity, "network_bind": "loopback-only", "privileged": device.IsPrivileged(), "time": time.Now().UTC()})
}

func (r *runtimeEnv) hostAPI(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	host, err := device.InspectHost()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, host)
}

func (r *runtimeEnv) devicesAPI(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	items, err := device.Inventory()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": items, "count": len(items)})
}

func (r *runtimeEnv) jobsAPI(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	jobs, err := r.state.RecentJobs(50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

type scanRequest struct {
	Scope  scope.Scope `json:"scope"`
	Target string      `json:"target"`
	Port   int         `json:"port"`
	URL    string      `json:"url"`
}

func (r *runtimeEnv) scanAPI(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	operationName := strings.TrimPrefix(request.URL.Path, "/api/scan/")
	operation, ok := map[string]scope.Operation{"tcp_connect": scope.OperationTCP, "tls_metadata": scope.OperationTLS, "http_metadata": scope.OperationHTTP}[operationName]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown guarded diagnostic"})
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	var input scanRequest
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	if err := input.Scope.Validate(time.Now().UTC()); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	if operation == scope.OperationHTTP {
		if input.URL == "" {
			input.URL = "https://" + input.Target + fmt.Sprintf(":%d/", input.Port)
		}
		host, port, err := scan.HostPort(input.URL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		input.Target, input.Port = host, port
	}
	if input.Target == "" || input.Port < 1 || input.Port > 65535 {
		writeError(w, http.StatusBadRequest, errors.New("target and valid port are required"))
		return
	}
	result := r.run(operation, input.Scope, input.Target, input.Port, input.URL)
	code := http.StatusOK
	if result.Status == "BLOCKED" {
		code = http.StatusForbidden
	}
	if result.Status == "FAILED" {
		code = http.StatusBadGateway
	}
	writeJSON(w, code, result)
}

type agentChatRequest struct {
	Message string `json:"message"`
}

func (r *runtimeEnv) agentStatusAPI(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	contextSummary, err := r.agent.Context()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready", "takeover_mode": r.agent.InTakeover(), "context": contextSummary, "write_token": r.writeToken, "capabilities": []string{"contextual_chat", "visible_activity", "approval_gated_local_actions", "local_memory", "skill_drafts", "endpoint_snapshot"}})
}

func (r *runtimeEnv) agentContextAPI(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	contextSummary, err := r.agent.Context()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, contextSummary)
}

func (r *runtimeEnv) agentChatAPI(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var input agentChatRequest
	if err := decodeJSON(w, request, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	context, cancel := context.WithTimeout(request.Context(), 15*time.Second)
	defer cancel()
	response, err := r.agent.Chat(context, input.Message)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (r *runtimeEnv) agentMessagesAPI(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	items, err := r.agent.Messages()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": items})
}

func (r *runtimeEnv) agentMemoriesAPI(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	items, err := r.agent.Memories()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"memories": items})
}

func (r *runtimeEnv) agentActivityAPI(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	items, err := r.agent.Activities()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"activity": items})
}

func (r *runtimeEnv) agentCommandsAPI(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"commands": agent.Catalog()})
}

func (r *runtimeEnv) agentCommandAPI(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	parts := strings.Split(strings.TrimPrefix(request.URL.Path, "/api/agent/commands/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "propose" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown agent command route"})
		return
	}
	task, err := r.agent.Propose(parts[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, task)
}

func (r *runtimeEnv) agentTasksAPI(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	items, err := r.agent.Tasks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": items})
}

func (r *runtimeEnv) agentTaskAPI(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	parts := strings.Split(strings.TrimPrefix(request.URL.Path, "/api/agent/tasks/"), "/")
	if len(parts) != 2 || parts[0] == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown agent task route"})
		return
	}
	var task store.AgentTask
	var err error
	switch parts[1] {
	case "approve":
		task, err = r.agent.ApproveAndRun(parts[0])
	case "cancel":
		task, err = r.agent.Cancel(parts[0])
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown task action"})
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (r *runtimeEnv) agentTakeoverAPI(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	action := strings.TrimSpace(request.URL.Query().Get("action"))
	var err error
	switch action {
	case "start":
		err = r.agent.StartTakeover()
	case "end":
		err = r.agent.EndTakeover()
	default:
		writeError(w, http.StatusBadRequest, errors.New("takeover action must be start or end"))
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"takeover_mode": r.agent.InTakeover()})
}

func (r *runtimeEnv) agentSkillsAPI(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	items, err := r.agent.Skills()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": items})
}

func (r *runtimeEnv) agentSkillAPI(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	parts := strings.Split(strings.TrimPrefix(request.URL.Path, "/api/agent/skills/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "approve" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown skill action"})
		return
	}
	item, err := r.agent.ApproveSkill(parts[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func decodeJSON(w http.ResponseWriter, request *http.Request, destination any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}

func (r *runtimeEnv) requireWriteToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost && request.Header.Get("X-Network-Ninja-Write-Token") != r.writeToken {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "missing or invalid local write token"})
			return
		}
		next.ServeHTTP(w, request)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; connect-src 'self'")
		next.ServeHTTP(w, request)
	})
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}
func mustJSON(value any, err error) {
	if err != nil {
		fatal(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		fatal(err)
	}
}
func fatal(err error) { fmt.Fprintln(os.Stderr, "ERROR:", err); os.Exit(1) }
