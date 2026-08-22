package scope

import (
	"testing"
	"time"
)

func validScope(now time.Time) Scope {
	return Scope{
		EngagementID: "lab-001", Operator: "test-operator", AuthorizedTargets: []string{"127.0.0.1", "10.10.0.0/24"}, ExcludedTargets: []string{"10.10.0.99"}, AllowedPorts: []int{443, 8443}, AllowedOperations: []Operation{OperationTCP, OperationTLS}, StartTime: now.Add(-time.Minute), ExpiryTime: now.Add(time.Hour), RateLimitPerMin: 10, NetworkBoundary: "isolated-test-lab", EvidenceLocation: "./evidence",
	}
}

func TestAuthorizePermitsExplicitTargetPortAndOperation(t *testing.T) {
	now := time.Now().UTC()
	decision := validScope(now).Authorize(OperationTCP, "127.0.0.1", 443, now)
	if !decision.Allowed {
		t.Fatalf("expected allow, got %q", decision.Reason)
	}
}

func TestAuthorizeRejectsOutOfScopeTarget(t *testing.T) {
	now := time.Now().UTC()
	decision := validScope(now).Authorize(OperationTCP, "192.0.2.20", 443, now)
	if decision.Allowed || decision.Reason != "target is outside authorized_targets" {
		t.Fatalf("expected target denial, got %#v", decision)
	}
}

func TestAuthorizeRejectsExcludedCIDRMember(t *testing.T) {
	now := time.Now().UTC()
	decision := validScope(now).Authorize(OperationTCP, "10.10.0.99", 443, now)
	if decision.Allowed || decision.Reason != "target is listed in excluded_targets" {
		t.Fatalf("expected exclusion denial, got %#v", decision)
	}
}

func TestAuthorizeRejectsUnapprovedPortAndOperation(t *testing.T) {
	now := time.Now().UTC()
	engagement := validScope(now)
	if decision := engagement.Authorize(OperationTCP, "127.0.0.1", 22, now); decision.Allowed {
		t.Fatal("expected port denial")
	}
	if decision := engagement.Authorize(OperationHTTP, "127.0.0.1", 443, now); decision.Allowed {
		t.Fatal("expected operation denial")
	}
}

func TestValidateRejectsExpiredScope(t *testing.T) {
	now := time.Now().UTC()
	engagement := validScope(now)
	engagement.ExpiryTime = now.Add(-time.Second)
	if err := engagement.Validate(now); err == nil {
		t.Fatal("expected expired scope validation failure")
	}
}
