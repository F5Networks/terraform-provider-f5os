package provider

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	f5os "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// newMockBackend creates a test HTTP server that satisfies the F5OS login
// handshake (returns X-Auth-Token and valid JSON for the aaa endpoint).
func newMockBackend() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Auth-Token", "test-token")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:aaa":{}}`))
	}))
}

// newSessionOrFail creates an F5OS session against the given backend URL,
// fatally failing the test if it cannot connect.
func newSessionOrFail(t *testing.T, backendURL string) *f5os.F5os {
	t.Helper()
	cfg := &f5os.F5osConfig{
		Host:             backendURL,
		User:             "admin",
		Password:         "admin",
		DisableSSLVerify: true,
	}
	session, err := f5os.NewSession(cfg)
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}
	return session
}

// =========================================================================
// Section 1: Transport wiring tests
// These verify that NewSession correctly configures the http.Transport
// with proxy and TLS settings.
// =========================================================================

// TestNewSession_TransportHasProxyFunc verifies that NewSession creates a
// transport whose Proxy field is non-nil (set to http.ProxyFromEnvironment).
func TestNewSession_TransportHasProxyFunc(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	session := newSessionOrFail(t, backend.URL)

	if session.Transport == nil {
		t.Fatal("expected Transport to be set on session, got nil")
	}
	if session.Transport.Proxy == nil {
		t.Fatal("expected Transport.Proxy to be set (http.ProxyFromEnvironment), got nil")
	}
}

// TestNewSession_TransportPreservesTLSAndProxy verifies that both TLS
// configuration and proxy configuration coexist on the transport.
func TestNewSession_TransportPreservesTLSAndProxy(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	session := newSessionOrFail(t, backend.URL)

	if session.Transport == nil {
		t.Fatal("session.Transport is nil")
	}
	if session.Transport.Proxy == nil {
		t.Fatal("session.Transport.Proxy is nil; proxy configuration missing")
	}
	if session.Transport.TLSClientConfig == nil {
		t.Fatal("session.Transport.TLSClientConfig is nil; TLS config lost")
	}
	if !session.Transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify=true when DisableSSLVerify is set")
	}
}

// TestNewSession_TLSVerifyPropagation verifies that the DisableSSLVerify
// flag correctly propagates to the transport's TLS config, and that proxy
// configuration is present in both secure and insecure modes.
func TestNewSession_TLSVerifyPropagation(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	// DisableSSLVerify = true
	cfgInsecure := &f5os.F5osConfig{
		Host:             backend.URL,
		User:             "admin",
		Password:         "admin",
		DisableSSLVerify: true,
	}
	sessionInsecure, err := f5os.NewSession(cfgInsecure)
	if err != nil {
		t.Fatalf("NewSession (insecure) failed: %v", err)
	}
	if !sessionInsecure.Transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify=true for DisableSSLVerify=true")
	}

	// DisableSSLVerify = false
	cfgSecure := &f5os.F5osConfig{
		Host:             backend.URL,
		User:             "admin",
		Password:         "admin",
		DisableSSLVerify: false,
	}
	sessionSecure, err := f5os.NewSession(cfgSecure)
	if err != nil {
		t.Fatalf("NewSession (secure) failed: %v", err)
	}
	if sessionSecure.Transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify=false for DisableSSLVerify=false")
	}

	// Both must have proxy configured.
	if sessionInsecure.Transport.Proxy == nil {
		t.Fatal("insecure session lost Proxy function")
	}
	if sessionSecure.Transport.Proxy == nil {
		t.Fatal("secure session lost Proxy function")
	}
}

// TestNewSession_MultipleSessionsHaveIndependentTransports verifies that
// each NewSession call creates its own Transport instance with proxy set.
func TestNewSession_MultipleSessionsHaveIndependentTransports(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	session1 := newSessionOrFail(t, backend.URL)
	session2 := newSessionOrFail(t, backend.URL)

	if session1.Transport == session2.Transport {
		t.Fatal("expected different Transport instances for different sessions")
	}
	if session1.Transport.Proxy == nil {
		t.Fatal("session1 Transport.Proxy is nil")
	}
	if session2.Transport.Proxy == nil {
		t.Fatal("session2 Transport.Proxy is nil")
	}
}

// TestNewSession_CustomPortPreservesProxy verifies that specifying a custom
// port in F5osConfig does not remove or reset the proxy configuration.
func TestNewSession_CustomPortPreservesProxy(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	cfg := &f5os.F5osConfig{
		Host:             backend.URL,
		User:             "admin",
		Password:         "admin",
		Port:             8888,
		DisableSSLVerify: true,
	}

	session, err := f5os.NewSession(cfg)
	if err != nil {
		t.Fatalf("NewSession with custom port failed: %v", err)
	}

	if session.Transport == nil {
		t.Fatal("Transport is nil with custom port")
	}
	if session.Transport.Proxy == nil {
		t.Fatal("Transport.Proxy is nil with custom port")
	}
}

// =========================================================================
// Section 2: Session field propagation tests
// These verify that the session stores all fields needed for proxy-aware
// transport reuse (e.g., in doRequest's 401 re-auth path).
// =========================================================================

// TestNewSession_SessionFieldsForTransportReuse verifies that the session
// contains all fields that doRequest copies into a new F5osConfig when
// re-authenticating on 401, ensuring proxy config survives re-auth.
func TestNewSession_SessionFieldsForTransportReuse(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	session := newSessionOrFail(t, backend.URL)

	if session.Transport == nil {
		t.Fatal("Transport is nil")
	}
	if session.Transport.Proxy == nil {
		t.Fatal("Transport.Proxy is nil; proxy config would be lost on re-auth")
	}
	if session.User != "admin" {
		t.Fatalf("expected User='admin', got %q", session.User)
	}
	if session.Password != "admin" {
		t.Fatalf("expected Password='admin', got %q", session.Password)
	}
	if !strings.HasPrefix(session.Host, "http") {
		t.Fatalf("expected Host to start with 'http', got %q", session.Host)
	}
	if session.ConfigOptions == nil {
		t.Fatal("ConfigOptions is nil; would cause panic in doRequest")
	}
}

// TestNewSession_TokenObtainedWithProxy verifies that NewSession
// successfully completes the login handshake (obtains auth token) when
// proxy configuration is present on the transport.
func TestNewSession_TokenObtainedWithProxy(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	session := newSessionOrFail(t, backend.URL)

	if session.Token == "" {
		t.Fatal("expected non-empty Token after successful login")
	}
	if session.Token != "test-token" {
		t.Fatalf("expected Token='test-token', got %q", session.Token)
	}
}

// =========================================================================
// Section 3: Transport.Proxy custom function tests
// Since http.ProxyFromEnvironment caches env vars via sync.Once, we cannot
// reliably modify env vars mid-process. Instead, we test that setting a
// custom Proxy function on the Transport works correctly -- which is the
// same mechanism used by http.ProxyFromEnvironment under the hood.
// These tests verify that the Transport's Proxy field is functional and
// correctly used by http.Client for routing decisions.
// =========================================================================

// TestTransportProxy_CustomProxyFuncRoutes verifies that an http.Client
// using a Transport with a custom Proxy function routes requests through
// the specified proxy server.
func TestTransportProxy_CustomProxyFuncRoutes(t *testing.T) {
	var proxyHitCount int

	// Start a mock target.
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`OK`))
	}))
	defer target.Close()

	// Start a forward proxy.
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHitCount++
		// Forward the request to the target.
		resp, err := http.Get(r.RequestURI)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		w.WriteHeader(resp.StatusCode)
		_, _ = fmt.Fprint(w, "proxied")
	}))
	defer proxy.Close()

	proxyURL, _ := url.Parse(proxy.URL)
	tr := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}
	client := &http.Client{Transport: tr}

	resp, err := client.Get(target.URL + "/test")
	if err != nil {
		t.Fatalf("GET through proxy failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if proxyHitCount == 0 {
		t.Fatal("expected request to hit the proxy, but it did not")
	}
}

// TestTransportProxy_NilProxyFuncBypassesProxy verifies that a Transport
// with Proxy returning nil routes directly to the target.
func TestTransportProxy_NilProxyFuncBypassesProxy(t *testing.T) {
	var targetHitCount int

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHitCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	tr := &http.Transport{
		Proxy: func(r *http.Request) (*url.URL, error) {
			return nil, nil // no proxy
		},
	}
	client := &http.Client{Transport: tr}

	_, err := client.Get(target.URL + "/test")
	if err != nil {
		t.Fatalf("GET without proxy failed: %v", err)
	}

	if targetHitCount == 0 {
		t.Fatal("expected request to hit the target directly")
	}
}

// TestTransportProxy_SelectiveProxyByHost verifies that a Proxy function
// can selectively route some hosts through a proxy and bypass others,
// similar to NO_PROXY behavior.
func TestTransportProxy_SelectiveProxyByHost(t *testing.T) {
	var proxyHitCount int

	// Start a target that always responds OK.
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHitCount++
		resp, err := http.Get(r.RequestURI)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		w.WriteHeader(resp.StatusCode)
	}))
	defer proxy.Close()

	targetURL, _ := url.Parse(target.URL)
	proxyURL, _ := url.Parse(proxy.URL)

	// Proxy function that bypasses "localhost" targets (simulating NO_PROXY).
	tr := &http.Transport{
		Proxy: func(r *http.Request) (*url.URL, error) {
			if r.URL.Host == targetURL.Host {
				return nil, nil // bypass
			}
			return proxyURL, nil // proxy everything else
		},
	}
	client := &http.Client{Transport: tr}

	// This request should bypass the proxy.
	_, err := client.Get(target.URL + "/direct")
	if err != nil {
		t.Fatalf("direct GET failed: %v", err)
	}
	if proxyHitCount != 0 {
		t.Fatalf("expected proxy to be bypassed for target host, but proxy was hit %d times", proxyHitCount)
	}
}

// TestTransportProxy_ReusedAfterNewSession verifies that the Transport
// created by NewSession (with Proxy set) would be correctly used by
// http.Client instances created in doRequest.
func TestTransportProxy_ReusedAfterNewSession(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	session := newSessionOrFail(t, backend.URL)

	// Simulate what doRequest does: create a new http.Client with the
	// session's Transport.
	client := &http.Client{
		Transport: session.Transport,
	}

	// The client should be functional.
	resp, err := client.Get(backend.URL + "/test")
	if err != nil {
		t.Fatalf("GET with session Transport failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	// And the transport should still have proxy configured.
	if session.Transport.Proxy == nil {
		t.Fatal("session.Transport.Proxy is nil after using it in an http.Client")
	}
}

// TestTransportProxy_ReauthPreservesTransport simulates the re-auth path
// in doRequest: when a 401 is received, a new F5osConfig is built from
// the session fields (including Transport). This test verifies the
// reconstruction preserves the proxy-configured Transport.
func TestTransportProxy_ReauthPreservesTransport(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	session := newSessionOrFail(t, backend.URL)

	// Simulate the F5osConfig reconstruction done in doRequest on 401.
	reconstructedCfg := &f5os.F5osConfig{
		Host:             session.Host,
		User:             session.User,
		Password:         session.Password,
		Transport:        session.Transport,
		DisableSSLVerify: session.DisableSSLVerify,
		ConfigOptions:    session.ConfigOptions,
		Port:             session.Port,
	}

	// Verify the reconstructed config carries the proxy-configured Transport.
	if reconstructedCfg.Transport == nil {
		t.Fatal("reconstructed F5osConfig.Transport is nil")
	}
	if reconstructedCfg.Transport.Proxy == nil {
		t.Fatal("reconstructed F5osConfig.Transport.Proxy is nil; proxy lost on re-auth")
	}

	// Verify we can create a new session from the reconstructed config
	// (this is what doRequest does on 401).
	newSession, err := f5os.NewSession(reconstructedCfg)
	if err != nil {
		t.Fatalf("NewSession from reconstructed config failed: %v", err)
	}
	if newSession.Transport == nil {
		t.Fatal("new session Transport is nil after re-auth")
	}
	if newSession.Transport.Proxy == nil {
		t.Fatal("new session Transport.Proxy is nil after re-auth")
	}
}

// =========================================================================
// Section 4: CustomHeaders tests
// These verify that custom HTTP headers are stored on the session,
// injected into API requests, and set as ProxyConnectHeader on the
// transport for HTTPS CONNECT tunnel injection.
// =========================================================================

// TestNewSession_CustomHeadersStoredOnSession verifies that headers
// provided in F5osConfig.CustomHeaders are copied onto the session.
func TestNewSession_CustomHeadersStoredOnSession(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	cfg := &f5os.F5osConfig{
		Host:             backend.URL,
		User:             "admin",
		Password:         "admin",
		DisableSSLVerify: true,
		CustomHeaders: map[string]string{
			"X-Tenant-ID":   "hsbc-prod",
			"X-Custom-Auth": "token123",
		},
	}
	session, err := f5os.NewSession(cfg)
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	if len(session.CustomHeaders) != 2 {
		t.Fatalf("expected 2 CustomHeaders on session, got %d", len(session.CustomHeaders))
	}
	if session.CustomHeaders["X-Tenant-ID"] != "hsbc-prod" {
		t.Fatalf("expected X-Tenant-ID='hsbc-prod', got %q", session.CustomHeaders["X-Tenant-ID"])
	}
	if session.CustomHeaders["X-Custom-Auth"] != "token123" {
		t.Fatalf("expected X-Custom-Auth='token123', got %q", session.CustomHeaders["X-Custom-Auth"])
	}
}

// TestNewSession_CustomHeadersSetProxyConnectHeader verifies that
// CustomHeaders are registered as ProxyConnectHeader on the transport,
// so they appear in the CONNECT tunnel request when using an HTTPS proxy.
func TestNewSession_CustomHeadersSetProxyConnectHeader(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	cfg := &f5os.F5osConfig{
		Host:             backend.URL,
		User:             "admin",
		Password:         "admin",
		DisableSSLVerify: true,
		CustomHeaders: map[string]string{
			"X-Tenant-ID": "hsbc-prod",
		},
	}
	session, err := f5os.NewSession(cfg)
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	if session.Transport.ProxyConnectHeader == nil {
		t.Fatal("expected ProxyConnectHeader to be set on transport when CustomHeaders provided")
	}
	if session.Transport.ProxyConnectHeader.Get("X-Tenant-ID") != "hsbc-prod" {
		t.Fatalf("expected ProxyConnectHeader X-Tenant-ID='hsbc-prod', got %q",
			session.Transport.ProxyConnectHeader.Get("X-Tenant-ID"))
	}
}

// TestNewSession_NoCustomHeadersLeavesProxyConnectHeaderNil verifies that
// when no CustomHeaders are provided, ProxyConnectHeader is not set.
func TestNewSession_NoCustomHeadersLeavesProxyConnectHeaderNil(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	session := newSessionOrFail(t, backend.URL)

	if session.Transport.ProxyConnectHeader != nil {
		t.Fatal("expected ProxyConnectHeader to be nil when no CustomHeaders provided")
	}
}

// TestNewSession_CustomHeadersSentOnLoginRequest verifies that custom
// headers are included in the initial login (NewSession) HTTP request.
func TestNewSession_CustomHeadersSentOnLoginRequest(t *testing.T) {
	var capturedHeaders http.Header
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture only the first request (the login request to uriLogin).
		if capturedHeaders == nil {
			capturedHeaders = r.Header.Clone()
		}
		w.Header().Set("X-Auth-Token", "test-token")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:aaa":{}}`))
	}))
	defer backend.Close()

	cfg := &f5os.F5osConfig{
		Host:             backend.URL,
		User:             "admin",
		Password:         "admin",
		DisableSSLVerify: true,
		CustomHeaders: map[string]string{
			"X-Tenant-ID": "hsbc-prod",
		},
	}
	_, err := f5os.NewSession(cfg)
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	if capturedHeaders.Get("X-Tenant-ID") != "hsbc-prod" {
		t.Fatalf("expected login request to contain X-Tenant-ID='hsbc-prod', got %q",
			capturedHeaders.Get("X-Tenant-ID"))
	}
}

// TestTransportProxy_CustomHeadersInCONNECTTunnel verifies end-to-end that
// headers placed in transport.ProxyConnectHeader actually appear in the
// CONNECT request sent to an HTTPS proxy — i.e. they are in the "outer"
// tunnel frame, before the encrypted payload. This is the core requirement
// for proxy authentication schemes that inspect CONNECT tunnel headers.
func TestTransportProxy_CustomHeadersInCONNECTTunnel(t *testing.T) {
	var (
		mu                     sync.Mutex
		capturedConnectHeaders http.Header
	)

	// Start a TLS target — the client must use CONNECT to reach HTTPS via a proxy.
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	// Start a plain HTTP proxy that handles CONNECT, captures its headers, and
	// bridges the connection so the TLS handshake completes.
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "only CONNECT supported", http.StatusMethodNotAllowed)
			return
		}
		mu.Lock()
		capturedConnectHeaders = r.Header.Clone()
		mu.Unlock()

		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijack not supported", http.StatusInternalServerError)
			return
		}
		clientConn, _, err := hj.Hijack()
		if err != nil {
			return
		}
		defer clientConn.Close()

		_, _ = fmt.Fprint(clientConn, "HTTP/1.1 200 Connection Established\r\n\r\n")

		targetConn, err := net.Dial("tcp", r.Host)
		if err != nil {
			return
		}
		defer targetConn.Close()

		done := make(chan struct{}, 2)
		go func() { _, _ = io.Copy(targetConn, clientConn); done <- struct{}{} }()
		go func() { _, _ = io.Copy(clientConn, targetConn); done <- struct{}{} }()
		<-done
	}))
	defer proxy.Close()

	proxyURL, _ := url.Parse(proxy.URL)
	tr := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		ProxyConnectHeader: http.Header{
			"X-Tenant-ID": {"hsbc-prod"},
		},
	}
	client := &http.Client{Transport: tr}

	resp, err := client.Get(target.URL + "/test")
	if err != nil {
		t.Fatalf("GET through HTTPS proxy failed: %v", err)
	}
	defer resp.Body.Close()

	mu.Lock()
	headers := capturedConnectHeaders
	mu.Unlock()

	if headers == nil {
		t.Fatal("proxy never received a CONNECT request")
	}
	if headers.Get("X-Tenant-ID") != "hsbc-prod" {
		t.Fatalf("expected CONNECT request to contain X-Tenant-ID='hsbc-prod', got %q",
			headers.Get("X-Tenant-ID"))
	}
}

// TestNewSession_ReauthPreservesCustomHeaders verifies that CustomHeaders
// survive the 401 re-auth path in doRequest, where a new F5osConfig is
// reconstructed from the session fields.
func TestNewSession_ReauthPreservesCustomHeaders(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	cfg := &f5os.F5osConfig{
		Host:             backend.URL,
		User:             "admin",
		Password:         "admin",
		DisableSSLVerify: true,
		CustomHeaders: map[string]string{
			"X-Tenant-ID": "hsbc-prod",
		},
	}
	session, err := f5os.NewSession(cfg)
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	// Simulate the F5osConfig reconstruction done in doRequest on 401.
	reconstructedCfg := &f5os.F5osConfig{
		Host:             session.Host,
		User:             session.User,
		Password:         session.Password,
		Transport:        session.Transport,
		DisableSSLVerify: session.DisableSSLVerify,
		ConfigOptions:    session.ConfigOptions,
		Port:             session.Port,
		CustomHeaders:    session.CustomHeaders,
	}

	if reconstructedCfg.CustomHeaders["X-Tenant-ID"] != "hsbc-prod" {
		t.Fatalf("expected X-Tenant-ID='hsbc-prod' in reconstructed config, got %q",
			reconstructedCfg.CustomHeaders["X-Tenant-ID"])
	}
}
