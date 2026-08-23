package agent

import (
	"context"
	"strings"
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
