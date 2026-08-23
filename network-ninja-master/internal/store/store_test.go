package store

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	state, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open state: %v", err)
	}
	t.Cleanup(func() { _ = state.Close() })
	return state
}

func TestMessageRetentionAndBounds(t *testing.T) {
	state := openTestStore(t)
	for index := 0; index < keepMessages+25; index++ {
		if _, err := state.AddMessage("user", fmt.Sprintf("message-%d", index)); err != nil {
			t.Fatalf("add message %d: %v", index, err)
		}
	}
	var count int
	if err := state.db.QueryRow(`SELECT COUNT(*) FROM conversation_messages`).Scan(&count); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if count != keepMessages {
		t.Fatalf("retention count=%d want=%d", count, keepMessages)
	}
	if _, err := state.AddMessage("system", "invalid role"); err == nil {
		t.Fatal("invalid role accepted")
	}
	if _, err := state.AddMessage("user", strings.Repeat("x", maxMessageBytes+1)); err == nil {
		t.Fatal("oversized message accepted")
	}
}

func TestTaskTransitionCompareAndSet(t *testing.T) {
	state := openTestStore(t)
	task, err := state.CreateTask(AgentTask{ID: "task-1", Kind: "LOCAL_COMMAND", Title: "Test", Status: "PENDING_APPROVAL"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	const workers = 16
	var group sync.WaitGroup
	outcomes := make(chan error, workers)
	for index := 0; index < workers; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := state.TransitionTask(task.ID, "PENDING_APPROVAL", "APPROVED", "", nil)
			outcomes <- err
		}()
	}
	group.Wait()
	close(outcomes)
	success := 0
	for err := range outcomes {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("transition winners=%d want=1", success)
	}
	stored, err := state.TaskByID(task.ID)
	if err != nil || stored.Status != "APPROVED" {
		t.Fatalf("stored task=%#v err=%v", stored, err)
	}
}
