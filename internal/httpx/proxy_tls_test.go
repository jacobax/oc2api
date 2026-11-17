package httpx

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

const tunneledBody = "tunneled-ok"

// selfSignedProxyCert mints a certificate whose SANs name no IP address, the
// shape of the residential https relays this gateway must still support: the
// strict proxy-leg handshake has to refuse it, tls_insecure=1 has to accept it.
func selfSignedProxyCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "self-signed-proxy"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"proxy.invalid"},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("load key pair: %v", err)
	}
	return cert
}

// serveConnectProxy answers CONNECT on a TLS listener whose certificate names
// no IP SAN, then pipes the tunnel to the target address.
func serveConnectProxy(t *testing.T, cert tls.Certificate) string {
	t.Helper()
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleConnectProxy(conn)
		}
	}()
	return listener.Addr().String()
}

func handleConnectProxy(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	request, err := http.ReadRequest(reader)
	if err != nil || request.Method != http.MethodConnect {
		return
	}
	upstream, err := net.DialTimeout("tcp", request.Host, 5*time.Second)
	if err != nil {
		return
	}
	defer upstream.Close()
	conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
	done := make(chan struct{})
	go func() { io.Copy(upstream, reader); close(done) }()
	io.Copy(conn, upstream)
	<-done
}

// startPlainTarget serves tunneledBody on a plain HTTP listener.
func startPlainTarget(t *testing.T) string {
	t.Helper()
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Write([]byte(tunneledBody))
		}),
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { server.Close() })
	go server.Serve(listener)
	return listener.Addr().String()
}

func bunThroughProxy(t *testing.T, target, proxy string) (*http.Response, error) {
	t.Helper()
	proxyURL, err := url.Parse(proxy)
	if err != nil {
		t.Fatalf("parse proxy: %v", err)
	}
	transport, err := NewBunTransport(BunOptions{Proxy: proxyURL, ConnectTimeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("build bun transport: %v", err)
	}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	return client.Get("http://" + target + "/")
}

// asProxyConnectError reports whether err unwinds to a *net.OpError, the shape
// the gateway classifies proxy failures by.
func asProxyConnectError(err error, target **net.OpError) bool {
	for err != nil {
		if op, ok := err.(*net.OpError); ok {
			*target = op
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

func TestHTTPSProxyWithSelfSignedCert(t *testing.T) {
	target := startPlainTarget(t)
	proxy := serveConnectProxy(t, selfSignedProxyCert(t))

	// Strict verification refuses a proxy certificate that names no IP SAN.
	resp, err := bunThroughProxy(t, target, "https://"+proxy)
	if err == nil {
		resp.Body.Close()
		t.Fatal("strict proxy verification accepted a certificate without an IP SAN")
	}
	var opErr *net.OpError
	if !asProxyConnectError(err, &opErr) || opErr.Op != "proxyconnect" {
		t.Fatalf("proxy failure is not classifiable as proxyconnect: %v", err)
	}

	// tls_insecure=1 skips only the proxy leg; the tunnel then carries plain
	// HTTP to the target.
	resp, err = bunThroughProxy(t, target, "https://"+proxy+"?tls_insecure=1")
	if err != nil {
		t.Fatalf("tls_insecure=1 dial failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != tunneledBody {
		t.Fatalf("tunneled response = %d %q, want 200 %q", resp.StatusCode, body, tunneledBody)
	}
}

func TestProxyTunnelDial(t *testing.T) {
	target := startPlainTarget(t)
	proxy := serveConnectProxy(t, selfSignedProxyCert(t))
	proxyURL, err := url.Parse("https://" + proxy + "?tls_insecure=1")
	if err != nil {
		t.Fatalf("parse proxy: %v", err)
	}
	conn, err := ProxyTunnelDial(context.Background(), &net.Dialer{Timeout: 5 * time.Second}, proxyURL, target)
	if err != nil {
		t.Fatalf("tunnel dial failed: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("GET / HTTP/1.1\r\nHost: " + target + "\r\nConnection: close\r\n\r\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var data []byte
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		data = append(data, buf[:n]...)
		if err != nil {
			break
		}
	}
	if !strings.Contains(string(data), tunneledBody) {
		t.Fatalf("tunnel carried no target reply; got %q", string(data[:min(len(data), 200)]))
	}
}
