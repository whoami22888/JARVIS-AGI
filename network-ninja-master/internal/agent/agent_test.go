package agent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/whoami22888/JARVIS-AGI/network-ninja-master/internal/store"
)

func openTestService(t *testing.T) *Service {
	t.Helper()
	state, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() { _ = state.Close() })
	return New(state)
}

func TestChatCreatesContextAndSkillDraft(t *testing.T) {
	service := openTestService(t)
	response, err := service.Chat(context.Background(), "Help me track what this computer is doing.")
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if response.Message == "" || response.Context.Messages != 2 {
		t.Fatalf("unexpected chat response: %#v", response)
	}
	draft, err := service.Chat(context.Background(), "skill: TLS posture review\nKeep certificate observations together.")
	if err != nil {
		t.Fatalf("draft skill: %v", err)
	}
	if draft.Skill == nil || draft.Skill.Status != "DRAFT" {
		t.Fatalf("expected draft skill, got %#v", draft.Skill)
	}
	skills, err := service.Skills()
	if err != nil || len(skills) != 1 || skills[0].Name != "TLS posture review" {
		t.Fatalf("skills: %#v err=%v", skills, err)
	}
}

func TestApprovalCancellationAndTakeover(t *testing.T) {
	service := openTestService(t)
	pending, err := service.Propose("host_inventory")
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if pending.Status != StatusPending {
		t.Fatalf("expected pending, got %s", pending.Status)
	}
	cancelled, err := service.Cancel(pending.ID)
	if err != nil || cancelled.Status != StatusCanceled {
		t.Fatalf("cancel: %#v err=%v", cancelled, err)
	}
	if _, err := service.ApproveAndRun(pending.ID); err == nil {
		t.Fatal("cancelled task unexpectedly approved")
	}

	approved, err := service.Propose("device_inventory")
	if err != nil {
		t.Fatalf("propose device inventory: %v", err)
	}
	completed, err := service.ApproveAndRun(approved.ID)
	if err != nil {
		t.Fatalf("approve and run: %v", err)
	}
	if completed.Status != StatusComplete || len(completed.Result) == 0 {
		t.Fatalf("expected completed task with result, got %#v", completed)
	}

	if err := service.StartTakeover(); err != nil {
		t.Fatalf("start takeover: %v", err)
	}
	if !service.InTakeover() {
		t.Fatal("takeover should be active")
	}
	if _, err := service.Propose("monitor_snapshot"); err == nil {
		t.Fatal("proposal unexpectedly allowed during takeover")
	}
	if err := service.EndTakeover(); err != nil {
		t.Fatalf("end takeover: %v", err)
	}
	if service.InTakeover() {
		t.Fatal("takeover should be disabled")
	}
}

func TestMonitorSnapshotCreatesTimelineEvent(t *testing.T) {
	service := openTestService(t)
	task, err := service.Propose("monitor_snapshot")
	if err != nil {
		t.Fatalf("propose snapshot: %v", err)
	}
	completed, err := service.ApproveAndRun(task.ID)
	if err != nil {
		t.Fatalf("run snapshot: %v", err)
	}
	if completed.Status != StatusComplete || !strings.Contains(string(completed.Result), "device_count") {
		t.Fatalf("unexpected monitor result: %#v", completed)
	}
	activity, err := service.Activities()
	if err != nil {
		t.Fatalf("activity: %v", err)
	}
	found := false
	for _, item := range activity {
		if item.Kind == "monitor" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("monitor activity not found: %#v", activity)
	}
}

func TestReasonerEgressIsConstrained(t *testing.T) {
	service := openTestService(t)
	if _, err := service.state.AddMessage("user", "prior secret=do-not-send"); err != nil {
		t.Fatalf("seed message: %v", err)
	}
	service.endpoint = "http://example.invalid/v1/chat/completions"
	if _, ok := service.optionalReasoner(context.Background(), "current request"); ok {
		t.Fatal("non-loopback HTTP endpoint was accepted")
	}

	var received string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		received = string(body)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Concise remote response"}}]}`))
	}))
	defer server.Close()
	service.endpoint = server.URL
	response, ok := service.optionalReasoner(context.Background(), "current request")
	if !ok || response != "Concise remote response" {
		t.Fatalf("expected bounded loopback reasoner response, got %q ok=%v", response, ok)
	}
	if strings.Contains(received, "do-not-send") {
		t.Fatalf("prior context leaked without opt-in: %s", received)
	}

	oversized := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"` + strings.Repeat("x", maxRemoteReplyBytes+1) + `"}}]}`))
	}))
	defer oversized.Close()
	service.endpoint = oversized.URL
	if _, ok := service.optionalReasoner(context.Background(), "current request"); ok {
		t.Fatal("oversized remote reasoner response was accepted")
	}

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		http.Redirect(w, request, "/other", http.StatusFound)
	}))
	defer redirect.Close()
	service.endpoint = redirect.URL
	if _, ok := service.optionalReasoner(context.Background(), "current request"); ok {
		t.Fatal("redirecting reasoner endpoint was accepted")
	}
}

func TestConcurrentProposalsHaveUniqueIDs(t *testing.T) {
	service := openTestService(t)
	const workers = 32
	ids := make(chan string, workers)
	errs := make(chan error, workers)
	var group sync.WaitGroup
	for i := 0; i < workers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			task, err := service.Propose("host_inventory")
			if err != nil {
				errs <- err
				return
			}
			ids <- task.ID
		}()
	}
	group.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("parallel proposal failed: %v", err)
		}
	}
	seen := map[string]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate task id %q", id)
		}
		seen[id] = true
	}
	if len(seen) != workers {
		t.Fatalf("expected %d task ids, got %d", workers, len(seen))
	}
}

func TestConcurrentApprovalHasOneWinner(t *testing.T) {
	service := openTestService(t)
	task, err := service.Propose("host_inventory")
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	const workers = 12
	results := make(chan error, workers)
	var group sync.WaitGroup
	for i := 0; i < workers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := service.ApproveAndRun(task.ID)
			results <- err
		}()
	}
	group.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("expected one approval winner, got %d", success)
	}
	stored, err := service.state.TaskByID(task.ID)
	if err != nil || stored.Status != StatusComplete {
		t.Fatalf("final task state: %#v err=%v", stored, err)
	}
}
