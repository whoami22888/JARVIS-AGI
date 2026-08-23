// Package agent implements the local, human-approved orchestration layer.
// It deliberately accepts typed commands only. Chat output can propose work but
// never turns arbitrary text into shell execution.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/whoami22888/JARVIS-AGI/network-ninja-master/internal/device"
	"github.com/whoami22888/JARVIS-AGI/network-ninja-master/internal/store"
)

const (
	StatusPending       = "PENDING_APPROVAL"
	StatusApproved      = "APPROVED"
	StatusRunning       = "RUNNING"
	StatusComplete      = "COMPLETED"
	StatusFailed        = "FAILED"
	StatusCanceled      = "CANCELLED"
	StatusTakeover      = "TAKEOVER"
	maxRemoteReplyBytes = 4096
)

var eventSequence atomic.Uint64

type Command struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	Risk             string `json:"risk"`
	RequiresApproval bool   `json:"requires_approval"`
	TerminalPreview  string `json:"terminal_preview"`
}

type ContextSummary struct {
	Messages   int      `json:"messages"`
	Memories   int      `json:"memories"`
	Activities int      `json:"activities"`
	Pending    int      `json:"pending_approvals"`
	Layers     []string `json:"layers"`
	Mode       string   `json:"mode"`
	Takeover   bool     `json:"takeover_mode"`
}

type ChatResponse struct {
	Message      string           `json:"message"`
	Context      ContextSummary   `json:"context"`
	Proposal     *store.AgentTask `json:"proposal,omitempty"`
	Skill        *store.Skill     `json:"skill,omitempty"`
	TakeoverMode bool             `json:"takeover_mode"`
}

type Service struct {
	state              *store.Store
	endpoint           string
	apiKey             string
	allowContextEgress bool
	client             *http.Client
}

func New(state *store.Store) *Service {
	return &Service{
		state:              state,
		endpoint:           strings.TrimSpace(os.Getenv("NETWORK_NINJA_CHAT_ENDPOINT")),
		apiKey:             strings.TrimSpace(os.Getenv("NETWORK_NINJA_CHAT_API_KEY")),
		allowContextEgress: os.Getenv("NETWORK_NINJA_CHAT_ALLOW_CONTEXT_EGRESS") == "1",
		client: &http.Client{
			Timeout:       12 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func Catalog() []Command {
	return []Command{
		{ID: "host_inventory", Title: "Inspect this computer", Description: "Collect local host, CPU, memory, OS, and network-interface facts through the embedded inventory collector.", Risk: "LOW", RequiresApproval: true, TerminalPreview: "network-ninja host inspect"},
		{ID: "device_inventory", Title: "Inspect attached devices", Description: "Collect read-only USB and serial inventory exposed by the local operating system.", Risk: "LOW", RequiresApproval: true, TerminalPreview: "network-ninja device inventory"},
		{ID: "monitor_snapshot", Title: "Capture an endpoint snapshot", Description: "Record fresh host and local-device inventory in the visible activity timeline.", Risk: "LOW", RequiresApproval: true, TerminalPreview: "network-ninja monitor snapshot"},
	}
}

func commandByID(id string) (Command, bool) {
	for _, command := range Catalog() {
		if command.ID == id {
			return command, true
		}
	}
	return Command{}, false
}

func eventID(kind string) string {
	return fmt.Sprintf("%s-%s-%06d", kind, time.Now().UTC().Format("20060102T150405.000000000Z"), eventSequence.Add(1))
}

func (s *Service) Context() (ContextSummary, error) {
	messages, err := s.state.RecentMessages(40)
	if err != nil {
		return ContextSummary{}, err
	}
	memories, err := s.state.RecentMemories(20)
	if err != nil {
		return ContextSummary{}, err
	}
	activities, err := s.state.RecentActivities(40)
	if err != nil {
		return ContextSummary{}, err
	}
	tasks, err := s.state.RecentTasks(80)
	if err != nil {
		return ContextSummary{}, err
	}
	pending := 0
	for _, task := range tasks {
		if task.Status == StatusPending || task.Status == StatusApproved {
			pending++
		}
	}
	mode := "LOCAL_CONTEXT_ACCELERATOR"
	if s.endpoint != "" {
		mode = "LOCAL_CONTEXT_PLUS_OPTIONAL_REASONER"
	}
	return ContextSummary{Messages: len(messages), Memories: len(memories), Activities: len(activities), Pending: pending, Layers: []string{"conversation", "durable-memory", "activity-timeline", "task-state", "skill-catalog"}, Mode: mode, Takeover: s.InTakeover()}, nil
}

func (s *Service) InTakeover() bool {
	memories, err := s.state.RecentMemories(20)
	if err != nil {
		return false
	}
	for _, item := range memories {
		if item.Category == "runtime" && item.MemoryKey == "takeover" {
			return item.Value == "enabled"
		}
	}
	return false
}

func (s *Service) setTakeover(enabled bool) error {
	value := "disabled"
	if enabled {
		value = "enabled"
	}
	_, err := s.state.UpsertMemory(store.Memory{Category: "runtime", MemoryKey: "takeover", Value: value, Source: "operator-console", Confidence: 1})
	return err
}

func (s *Service) StartTakeover() error {
	tasks, err := s.state.RecentTasks(200)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if task.Status == StatusPending || task.Status == StatusApproved {
			if _, err := s.state.TransitionTask(task.ID, task.Status, StatusTakeover, "operator takeover enabled", nil); err != nil {
				return err
			}
		}
	}
	if err := s.setTakeover(true); err != nil {
		return err
	}
	_, err = s.recordActivity("takeover", "Operator takeover enabled; pending agent actions were paused.", map[string]any{"paused_tasks": len(tasks)})
	return err
}

func (s *Service) EndTakeover() error {
	if err := s.setTakeover(false); err != nil {
		return err
	}
	_, err := s.recordActivity("takeover", "Operator takeover ended. New actions require a fresh approval.", nil)
	return err
}

func (s *Service) recordActivity(kind, summary string, detail any) (store.Activity, error) {
	encoded, err := json.Marshal(detail)
	if err != nil {
		return store.Activity{}, err
	}
	return s.state.RecordActivity(store.Activity{ID: eventID(kind), Kind: kind, Summary: summary, Detail: encoded})
}

func (s *Service) Propose(commandID string) (store.AgentTask, error) {
	command, ok := commandByID(commandID)
	if !ok {
		return store.AgentTask{}, fmt.Errorf("unknown command %q", commandID)
	}
	if s.InTakeover() {
		return store.AgentTask{}, errors.New("operator takeover is enabled; create a new proposal after takeover ends")
	}
	task, err := s.state.CreateTask(store.AgentTask{ID: eventID("task"), Kind: "LOCAL_COMMAND", Title: command.Title, CommandID: command.ID, Status: StatusPending})
	if err != nil {
		return store.AgentTask{}, err
	}
	_, _ = s.recordActivity("proposal", "Awaiting your approval: "+command.Title, map[string]any{"task_id": task.ID, "command_id": command.ID, "terminal_preview": command.TerminalPreview})
	return task, nil
}

func (s *Service) Cancel(taskID string) (store.AgentTask, error) {
	task, err := s.state.TaskByID(taskID)
	if err != nil {
		return store.AgentTask{}, err
	}
	if task.Status != StatusPending && task.Status != StatusApproved {
		return store.AgentTask{}, fmt.Errorf("task %s cannot be cancelled from %s", taskID, task.Status)
	}
	task, err = s.state.TransitionTask(taskID, task.Status, StatusCanceled, "operator cancelled", nil)
	if err == nil {
		_, _ = s.recordActivity("task", "Operator cancelled: "+task.Title, map[string]any{"task_id": task.ID})
	}
	return task, err
}

func (s *Service) ApproveAndRun(taskID string) (store.AgentTask, error) {
	if s.InTakeover() {
		return store.AgentTask{}, errors.New("operator takeover is enabled")
	}
	task, err := s.state.TaskByID(taskID)
	if err != nil {
		return store.AgentTask{}, err
	}
	if task.Status != StatusPending {
		return store.AgentTask{}, fmt.Errorf("task %s must be %s, got %s", taskID, StatusPending, task.Status)
	}
	if _, ok := commandByID(task.CommandID); !ok {
		return store.AgentTask{}, fmt.Errorf("task command %q is not in the approved catalog", task.CommandID)
	}
	if _, err = s.state.TransitionTask(taskID, StatusPending, StatusApproved, "", nil); err != nil {
		return store.AgentTask{}, err
	}
	if _, err = s.state.TransitionTask(taskID, StatusApproved, StatusRunning, "", nil); err != nil {
		return store.AgentTask{}, err
	}
	_, _ = s.recordActivity("approval", "Operator approved: "+task.Title, map[string]any{"task_id": task.ID, "command_id": task.CommandID})
	result, runErr := s.execute(task.CommandID)
	if runErr != nil {
		task, err = s.state.TransitionTask(taskID, StatusRunning, StatusFailed, runErr.Error(), map[string]any{"error": runErr.Error()})
		if err == nil {
			_, _ = s.recordActivity("task", "Task failed: "+task.Title, map[string]any{"task_id": task.ID, "error": runErr.Error()})
		}
		return task, err
	}
	task, err = s.state.TransitionTask(taskID, StatusRunning, StatusComplete, "", result)
	if err == nil {
		_, _ = s.recordActivity("task", "Task completed: "+task.Title, map[string]any{"task_id": task.ID, "command_id": task.CommandID})
	}
	return task, err
}

func (s *Service) execute(commandID string) (any, error) {
	switch commandID {
	case "host_inventory":
		return device.InspectHost()
	case "device_inventory":
		items, err := device.Inventory()
		return map[string]any{"devices": items, "count": len(items)}, err
	case "monitor_snapshot":
		host, err := device.InspectHost()
		if err != nil {
			return nil, err
		}
		items, err := device.Inventory()
		if err != nil {
			return nil, err
		}
		payload := map[string]any{"host": host, "devices": items, "device_count": len(items), "captured_at": time.Now().UTC()}
		_, _ = s.recordActivity("monitor", "Captured a fresh endpoint snapshot.", payload)
		return payload, nil
	default:
		return nil, fmt.Errorf("command %q is not executable", commandID)
	}
}

func (s *Service) Chat(ctx context.Context, input string) (ChatResponse, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return ChatResponse{}, errors.New("chat message is required")
	}
	if len(input) > 12000 {
		return ChatResponse{}, errors.New("chat message exceeds 12000 characters")
	}
	if _, err := s.state.AddMessage("user", input); err != nil {
		return ChatResponse{}, err
	}
	lower := strings.ToLower(input)
	response := ChatResponse{TakeoverMode: s.InTakeover()}
	if strings.HasPrefix(lower, "skill:") || strings.HasPrefix(lower, "upgrade:") {
		body := strings.TrimSpace(input[strings.Index(input, ":")+1:])
		if body == "" {
			return ChatResponse{}, errors.New("a skill or upgrade draft needs a description")
		}
		name := firstLine(body, 72)
		skill, err := s.state.CreateSkill(store.Skill{ID: eventID("skill"), Name: name, Description: "Operator-submitted chat upgrade draft", Body: body, Status: "DRAFT", Source: "chat"})
		if err != nil {
			return ChatResponse{}, err
		}
		response.Skill = &skill
		response.Message = "I saved that as a draft skill. It is visible in Skills, stays inactive, and can be reviewed or approved without automatically installing or running anything."
		_, _ = s.recordActivity("skill", "Draft skill added from chat: "+skill.Name, map[string]any{"skill_id": skill.ID})
	} else if commandID := inferredCommand(lower); commandID != "" {
		task, err := s.Propose(commandID)
		if err != nil {
			return ChatResponse{}, err
		}
		response.Proposal = &task
		response.Message = "I have prepared a local action card for you. Review the terminal preview and approve it when you want me to run the bounded collector. I will not run it before that."
	} else if strings.Contains(lower, "take over") || strings.Contains(lower, "takeover") || strings.Contains(lower, "pause agent") {
		if err := s.StartTakeover(); err != nil {
			return ChatResponse{}, err
		}
		response.TakeoverMode = true
		response.Message = "You are in control now. I paused pending work and will wait for a new request after you end takeover mode."
	} else if strings.Contains(lower, "resume") || strings.Contains(lower, "end takeover") {
		if err := s.EndTakeover(); err != nil {
			return ChatResponse{}, err
		}
		response.TakeoverMode = false
		response.Message = "Takeover mode is off. I am ready to prepare a fresh, visible action for your approval."
	} else {
		response.Message = s.contextualReply(ctx, input)
	}
	if _, err := s.state.AddMessage("assistant", response.Message); err != nil {
		return ChatResponse{}, err
	}
	contextSummary, err := s.Context()
	if err != nil {
		return ChatResponse{}, err
	}
	response.Context = contextSummary
	return response, nil
}

func inferredCommand(lower string) string {
	switch {
	case strings.Contains(lower, "monitor"), strings.Contains(lower, "snapshot"), strings.Contains(lower, "what is this computer doing"):
		return "monitor_snapshot"
	case strings.Contains(lower, "usb"), strings.Contains(lower, "serial"), strings.Contains(lower, "device inventory"):
		return "device_inventory"
	case strings.Contains(lower, "host inventory"), strings.Contains(lower, "inspect computer"), strings.Contains(lower, "system information"):
		return "host_inventory"
	default:
		return ""
	}
}

func firstLine(text string, limit int) string {
	line := strings.TrimSpace(strings.Split(text, "\n")[0])
	if len(line) > limit {
		return strings.TrimSpace(line[:limit])
	}
	return line
}

func (s *Service) contextualReply(ctx context.Context, input string) string {
	contextSummary, err := s.Context()
	if err != nil {
		return "I could not read local context right now. Please refresh the console and try again."
	}
	if contextSummary.Takeover {
		return "I am standing by while takeover mode is active. When you are ready, end takeover and I can prepare a fresh action card."
	}
	if remote, ok := s.optionalReasoner(ctx, input); ok {
		return remote
	}
	return fmt.Sprintf("Got it. I am using %d recent chat messages, %d durable notes, and %d timeline events to keep the thread together. I can explain the current state, draft a skill with `skill:`, or prepare a visible local inventory action for your approval.", contextSummary.Messages, contextSummary.Memories, contextSummary.Activities)
}

// optionalReasoner is text-only and never receives tools or execution authority.
// If the configured endpoint is absent, unavailable, or malformed, the local
// context accelerator remains fully functional.
func (s *Service) optionalReasoner(ctx context.Context, input string) (string, bool) {
	endpoint, ok := safeReasonerEndpoint(s.endpoint)
	if !ok {
		return "", false
	}
	contextLines := []string{"Latest request:\n" + input}
	if s.allowContextEgress {
		messages, err := s.state.RecentMessages(12)
		if err != nil {
			return "", false
		}
		memory, err := s.state.RecentMemories(8)
		if err != nil {
			return "", false
		}
		contextLines = make([]string, 0, len(messages)+len(memory)+1)
		for _, item := range memory {
			contextLines = append(contextLines, "memory: "+item.MemoryKey+" = "+item.Value)
		}
		for _, item := range messages {
			contextLines = append(contextLines, item.Role+": "+item.Content)
		}
		contextLines = append(contextLines, "Latest request:\n"+input)
	}
	payload := map[string]any{"messages": []map[string]string{{"role": "system", "content": "You are a concise, collaborative local operations coworker. You have no authority to execute commands. Explain that any consequential action needs a visible human approval."}, {"role": "user", "content": strings.Join(contextLines, "\n")}}, "max_tokens": 280}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return "", false
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", false
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 128<<10)).Decode(&decoded); err != nil || len(decoded.Choices) == 0 {
		return "", false
	}
	text := strings.TrimSpace(decoded.Choices[0].Message.Content)
	if text == "" || len(text) > maxRemoteReplyBytes {
		return "", false
	}
	return text, true
}

func safeReasonerEndpoint(raw string) (*url.URL, bool) {
	if raw == "" {
		return nil, false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Hostname() == "" {
		return nil, false
	}
	if parsed.Scheme == "https" {
		return parsed, true
	}
	if parsed.Scheme != "http" {
		return nil, false
	}
	host := strings.Trim(parsed.Hostname(), "[]")
	ip := net.ParseIP(host)
	return parsed, host == "localhost" || (ip != nil && ip.IsLoopback())
}

func (s *Service) Messages() ([]store.ConversationMessage, error) { return s.state.RecentMessages(80) }
func (s *Service) Memories() ([]store.Memory, error)              { return s.state.RecentMemories(40) }
func (s *Service) Activities() ([]store.Activity, error)          { return s.state.RecentActivities(100) }
func (s *Service) Tasks() ([]store.AgentTask, error)              { return s.state.RecentTasks(100) }
func (s *Service) Skills() ([]store.Skill, error)                 { return s.state.RecentSkills(60) }
func (s *Service) ApproveSkill(id string) (store.Skill, error) {
	item, err := s.state.SetSkillStatus(id, "APPROVED")
	if err == nil {
		_, _ = s.recordActivity("skill", "Operator approved inert skill knowledge: "+item.Name, map[string]any{"skill_id": item.ID})
	}
	return item, err
}
