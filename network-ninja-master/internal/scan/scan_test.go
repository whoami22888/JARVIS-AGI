package scan

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestTCPAgainstRealLoopbackListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			connection.Close()
		}
	}()
	port := listener.Addr().(*net.TCPAddr).Port
	result := TCP(context.Background(), "127.0.0.1", port)
	if !result.Reachable || result.Error != "" {
		t.Fatalf("expected reachable TCP result, got %#v", result)
	}
}

func TestHTTPHeadAgainstRealLoopbackServer(t *testing.T) {
	server := http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("X-Network-Ninja-Test", "present")
		w.WriteHeader(http.StatusNoContent)
	})}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go server.Serve(listener)
	defer server.Close()
	result := HTTP(context.Background(), "http://"+listener.Addr().String()+"/")
	if result.Error != "" || result.StatusCode != http.StatusNoContent || result.Headers["X-Network-Ninja-Test"][0] != "present" {
		t.Fatalf("unexpected HTTP result: %#v", result)
	}
}

func TestTLSMetadataAgainstRealLoopbackServer(t *testing.T) {
	certificate := testCertificate(t)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{certificate}})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			if tlsConnection, ok := connection.(*tls.Conn); ok {
				_ = tlsConnection.Handshake()
			}
			_ = connection.Close()
		}
	}()
	port := listener.Addr().(*net.TCPAddr).Port
	result := TLS(context.Background(), "localhost", port)
	if result.Error != "" || !result.HandshakeComplete || result.Certificate == nil {
		t.Fatalf("unexpected TLS result: %#v", result)
	}
	if result.TrustedByHost {
		t.Fatal("test self-signed certificate must not be treated as trusted")
	}
}

func testCertificate(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	encoded, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: encoded})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return certificate
}
