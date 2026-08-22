// Package scan contains low-impact, evidence-oriented diagnostic primitives.
// It intentionally has no payload delivery, credential handling, remote shell,
// or arbitrary command execution capability.
package scan

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type TCPResult struct {
	Target     string        `json:"target"`
	Port       int           `json:"port"`
	Reachable  bool          `json:"reachable"`
	RemoteAddr string        `json:"remote_addr,omitempty"`
	Duration   time.Duration `json:"duration_ns"`
	Error      string        `json:"error,omitempty"`
	ObservedAt time.Time     `json:"observed_at"`
}

type TLSResult struct {
	Target            string        `json:"target"`
	Port              int           `json:"port"`
	HandshakeComplete bool          `json:"handshake_complete"`
	Protocol          string        `json:"protocol,omitempty"`
	CipherSuite       string        `json:"cipher_suite,omitempty"`
	ServerName        string        `json:"server_name"`
	Certificate       *Certificate  `json:"certificate,omitempty"`
	TrustedByHost     bool          `json:"trusted_by_host"`
	Duration          time.Duration `json:"duration_ns"`
	Error             string        `json:"error,omitempty"`
	ObservedAt        time.Time     `json:"observed_at"`
}

type Certificate struct {
	Subject       string    `json:"subject"`
	Issuer        string    `json:"issuer"`
	SerialNumber  string    `json:"serial_number"`
	NotBefore     time.Time `json:"not_before"`
	NotAfter      time.Time `json:"not_after"`
	DNSNames      []string  `json:"dns_names,omitempty"`
	SignatureAlgo string    `json:"signature_algorithm"`
	PublicKeyAlgo string    `json:"public_key_algorithm"`
}

type HTTPResult struct {
	URL        string              `json:"url"`
	StatusCode int                 `json:"status_code,omitempty"`
	Status     string              `json:"status,omitempty"`
	Headers    map[string][]string `json:"headers,omitempty"`
	RedirectTo string              `json:"redirect_to,omitempty"`
	Duration   time.Duration       `json:"duration_ns"`
	Error      string              `json:"error,omitempty"`
	ObservedAt time.Time           `json:"observed_at"`
}

func TCP(ctx context.Context, target string, port int) TCPResult {
	started := time.Now()
	result := TCPResult{Target: target, Port: port, ObservedAt: started.UTC()}
	dialer := net.Dialer{Timeout: deadline(ctx, 5*time.Second)}
	connection, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(target, strconv.Itoa(port)))
	result.Duration = time.Since(started)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer connection.Close()
	result.Reachable = true
	result.RemoteAddr = connection.RemoteAddr().String()
	return result
}

// TLS performs a metadata-only handshake. It deliberately does not transmit
// credentials or HTTP requests. Certificate fields are inspected and recorded;
// no unverified certificate is treated as trusted.
func TLS(ctx context.Context, target string, port int) TLSResult {
	started := time.Now()
	result := TLSResult{Target: target, Port: port, ServerName: target, ObservedAt: started.UTC()}
	dialer := &net.Dialer{Timeout: deadline(ctx, 7*time.Second)}
	connection, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(target, strconv.Itoa(port)), &tls.Config{ServerName: target, InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}) // #nosec G402 -- metadata inspection verifies separately below.
	result.Duration = time.Since(started)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer connection.Close()
	state := connection.ConnectionState()
	result.HandshakeComplete = state.HandshakeComplete
	result.Protocol = tls.VersionName(state.Version)
	result.CipherSuite = tls.CipherSuiteName(state.CipherSuite)
	if len(state.PeerCertificates) > 0 {
		leaf := state.PeerCertificates[0]
		result.Certificate = &Certificate{Subject: leaf.Subject.String(), Issuer: leaf.Issuer.String(), SerialNumber: leaf.SerialNumber.String(), NotBefore: leaf.NotBefore.UTC(), NotAfter: leaf.NotAfter.UTC(), DNSNames: leaf.DNSNames, SignatureAlgo: leaf.SignatureAlgorithm.String(), PublicKeyAlgo: leaf.PublicKeyAlgorithm.String()}
		result.TrustedByHost = verifyCertificate(leaf, state.PeerCertificates[1:], target)
	}
	return result
}

func verifyCertificate(leaf *x509.Certificate, intermediates []*x509.Certificate, serverName string) bool {
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		return false
	}
	pool := x509.NewCertPool()
	for _, certificate := range intermediates {
		pool.AddCert(certificate)
	}
	_, err = leaf.Verify(x509.VerifyOptions{DNSName: serverName, Roots: roots, Intermediates: pool, CurrentTime: time.Now()})
	return err == nil
}

// HTTP performs an HTTP HEAD request only. It follows no redirects, has a
// strict deadline, and captures response headers rather than document bodies.
func HTTP(ctx context.Context, rawURL string) HTTPResult {
	started := time.Now()
	result := HTTPResult{URL: rawURL, ObservedAt: started.UTC()}
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		result.Error = "URL must include an http or https scheme and hostname"
		result.Duration = time.Since(started)
		return result
	}
	client := &http.Client{Timeout: deadline(ctx, 8*time.Second), CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(started)
		return result
	}
	request.Header.Set("User-Agent", "Network-Ninja-Master/0.1 authorized-diagnostic")
	response, err := client.Do(request)
	result.Duration = time.Since(started)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer response.Body.Close()
	result.StatusCode, result.Status = response.StatusCode, response.Status
	result.Headers = cloneHeaders(response.Header)
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		result.RedirectTo = response.Header.Get("Location")
	}
	return result
}

func HostPort(rawURL string) (host string, port int, err error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return "", 0, fmt.Errorf("invalid URL")
	}
	host = parsed.Hostname()
	if explicit := parsed.Port(); explicit != "" {
		port, err = strconv.Atoi(explicit)
		return host, port, err
	}
	if strings.EqualFold(parsed.Scheme, "https") {
		return host, 443, nil
	}
	return host, 80, nil
}

func deadline(ctx context.Context, fallback time.Duration) time.Duration {
	if remaining, ok := ctx.Deadline(); ok {
		value := time.Until(remaining)
		if value > 0 && value < fallback {
			return value
		}
	}
	return fallback
}

func cloneHeaders(source http.Header) map[string][]string {
	result := make(map[string][]string, len(source))
	for key, values := range source {
		result[key] = append([]string(nil), values...)
	}
	return result
}
