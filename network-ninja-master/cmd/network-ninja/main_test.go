package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentWritesRequireRuntimeToken(t *testing.T) {
	runtime, err := openRuntime(t.TempDir())
	if err != nil {
		t.Fatalf("open runtime: %v", err)
	}
	defer runtime.state.Close()
	server := httptest.NewServer(runtime.routes())
	defer server.Close()

	payload := []byte(`{"message":"hello"}`)
	response, err := http.Post(server.URL+"/api/agent/chat", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("post without token: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("status without token=%d want=%d", response.StatusCode, http.StatusForbidden)
	}

	status, err := http.Get(server.URL + "/api/agent/status")
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	defer status.Body.Close()
	var statusBody struct {
		WriteToken string `json:"write_token"`
	}
	if err := json.NewDecoder(status.Body).Decode(&statusBody); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if len(statusBody.WriteToken) < 32 {
		t.Fatal("runtime did not issue a strong write token")
	}

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/agent/chat", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("create token request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Network-Ninja-Write-Token", statusBody.WriteToken)
	accepted, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("post with token: %v", err)
	}
	defer accepted.Body.Close()
	if accepted.StatusCode != http.StatusOK {
		t.Fatalf("status with token=%d want=%d", accepted.StatusCode, http.StatusOK)
	}
}
