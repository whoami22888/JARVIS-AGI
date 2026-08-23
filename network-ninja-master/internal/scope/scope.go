// Package scope validates the explicit authorization object required before any
// active Network Ninja diagnostic is attempted.
package scope

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"
)

// Operation is an allowlisted, low-impact diagnostic category.
type Operation string

const (
	OperationTCP  Operation = "tcp_connect"
	OperationTLS  Operation = "tls_metadata"
	OperationHTTP Operation = "http_metadata"
)

var validOperations = map[Operation]struct{}{
	OperationTCP: {}, OperationTLS: {}, OperationHTTP: {},
}

// Scope is a time-bounded engagement definition. It deliberately contains no
// free-form command field and cannot authorize arbitrary command execution.
type Scope struct {
	EngagementID      string      `json:"engagement_id"`
	Operator          string      `json:"operator"`
	AuthorizedTargets []string    `json:"authorized_targets"`
	ExcludedTargets   []string    `json:"excluded_targets"`
	AllowedPorts      []int       `json:"allowed_ports"`
	AllowedOperations []Operation `json:"allowed_operations"`
	ForbiddenOps      []Operation `json:"forbidden_operations"`
	StartTime         time.Time   `json:"start_time"`
	ExpiryTime        time.Time   `json:"expiry_time"`
	RateLimitPerMin   int         `json:"rate_limit_per_min"`
	NetworkBoundary   string      `json:"network_boundary"`
	EvidenceLocation  string      `json:"evidence_location"`
}

// Decision records an authorization outcome that can be persisted without
// losing the precise refusal reason.
type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}

func Load(path string) (Scope, error) {
	// #nosec G304 -- path is supplied explicitly by the local CLI operator and parsed as a validated scope document.
	data, err := os.ReadFile(path)
	if err != nil {
		return Scope{}, fmt.Errorf("read scope: %w", err)
	}
	var s Scope
	if err := json.Unmarshal(data, &s); err != nil {
		return Scope{}, fmt.Errorf("parse scope JSON: %w", err)
	}
	if err := s.Validate(time.Now().UTC()); err != nil {
		return Scope{}, err
	}
	return s, nil
}

func (s Scope) Validate(now time.Time) error {
	if strings.TrimSpace(s.EngagementID) == "" {
		return errors.New("scope engagement_id is required")
	}
	if strings.TrimSpace(s.Operator) == "" {
		return errors.New("scope operator is required")
	}
	if len(s.AuthorizedTargets) == 0 {
		return errors.New("scope must contain at least one authorized target")
	}
	if s.StartTime.IsZero() || s.ExpiryTime.IsZero() {
		return errors.New("scope start_time and expiry_time are required")
	}
	if !s.ExpiryTime.After(s.StartTime) {
		return errors.New("scope expiry_time must be after start_time")
	}
	if now.Before(s.StartTime) {
		return errors.New("scope has not started")
	}
	if !now.Before(s.ExpiryTime) {
		return errors.New("scope has expired")
	}
	if len(s.AllowedOperations) == 0 {
		return errors.New("scope allowed_operations is required")
	}
	for _, operation := range s.AllowedOperations {
		if _, ok := validOperations[operation]; !ok {
			return fmt.Errorf("unsupported allowed operation %q", operation)
		}
	}
	for _, port := range s.AllowedPorts {
		if port < 1 || port > 65535 {
			return fmt.Errorf("invalid allowed port %d", port)
		}
	}
	if s.RateLimitPerMin < 1 || s.RateLimitPerMin > 120 {
		return errors.New("scope rate_limit_per_min must be between 1 and 120")
	}
	for _, target := range append(append([]string{}, s.AuthorizedTargets...), s.ExcludedTargets...) {
		if err := validateTarget(target); err != nil {
			return err
		}
	}
	return nil
}

func validateTarget(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return errors.New("scope target cannot be empty")
	}
	if _, err := netip.ParsePrefix(target); err == nil {
		return nil
	}
	if net.ParseIP(target) != nil {
		return nil
	}
	if len(target) > 253 || strings.ContainsAny(target, "/\\@ ") {
		return fmt.Errorf("invalid hostname target %q", target)
	}
	return nil
}

func (s Scope) Authorize(operation Operation, host string, port int, now time.Time) Decision {
	if err := s.Validate(now); err != nil {
		return Decision{Reason: err.Error()}
	}
	if _, ok := validOperations[operation]; !ok {
		return Decision{Reason: "operation is not implemented"}
	}
	if !containsOperation(s.AllowedOperations, operation) {
		return Decision{Reason: "operation is not included in allowed_operations"}
	}
	if containsOperation(s.ForbiddenOps, operation) {
		return Decision{Reason: "operation is explicitly forbidden"}
	}
	if !containsPort(s.AllowedPorts, port) {
		return Decision{Reason: fmt.Sprintf("port %d is not included in allowed_ports", port)}
	}
	if targetMatchesAny(host, s.ExcludedTargets) {
		return Decision{Reason: "target is listed in excluded_targets"}
	}
	if !targetMatchesAny(host, s.AuthorizedTargets) {
		return Decision{Reason: "target is outside authorized_targets"}
	}
	return Decision{Allowed: true, Reason: "authorized"}
}

func containsOperation(items []Operation, target Operation) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func containsPort(items []int, target int) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func targetMatchesAny(host string, targets []string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	ip := net.ParseIP(host)
	for _, raw := range targets {
		target := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(raw)), ".")
		if prefix, err := netip.ParsePrefix(target); err == nil && ip != nil {
			if parsed, ok := netip.AddrFromSlice(ip); ok && prefix.Contains(parsed.Unmap()) {
				return true
			}
			continue
		}
		if strings.EqualFold(host, target) {
			return true
		}
	}
	return false
}
