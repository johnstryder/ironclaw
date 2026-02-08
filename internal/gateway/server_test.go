package gateway

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ironclaw/internal/domain"
)

// isListenPermissionErr reports whether err is a listen/bind permission error (e.g. sandbox).
func isListenPermissionErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "operation not permitted") || strings.Contains(s, "permission denied")
}

// fakeListener is a net.Listener that never accepts; Accept blocks until Close. For testing Run() without binding.
type fakeListener struct {
	addr   net.Addr
	closed chan struct{}
}

func (f *fakeListener) Accept() (net.Conn, error) {
	<-f.closed
	return nil, net.ErrClosed
}
func (f *fakeListener) Close() error {
	close(f.closed)
	return nil
}
func (f *fakeListener) Addr() net.Addr {
	return f.addr
}

func TestServer_WhenAuthTokenSet_ShouldRequireBearer(t *testing.T) {
	cfg := &domain.GatewayConfig{
		Port: 0,
		Auth: domain.AuthConfig{AuthToken: "my-secret"},
	}
	srv, err := NewServer(cfg, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler := srv.Handler()

	// without token -> 401
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("without token: want 401, got %d", rec.Code)
	}

	// with wrong token -> 401
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Authorization", "Bearer wrong")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("wrong token: want 401, got %d", rec2.Code)
	}

	// with correct token -> 200
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.Header.Set("Authorization", "Bearer my-secret")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Errorf("correct token: want 200, got %d", rec3.Code)
	}
	if body := rec3.Body.String(); body != "OK" {
		t.Errorf("correct token body: want OK, got %q", body)
	}
}

func TestServer_WhenAuthTokenEmpty_ShouldAcceptRequestsWithoutHeader(t *testing.T) {
	cfg := &domain.GatewayConfig{
		Port: 0,
		Auth: domain.AuthConfig{},
	}
	srv, err := NewServer(cfg, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("no auth: want 200, got %d", rec.Code)
	}
	if rec.Body.String() != "OK" {
		t.Errorf("no auth body: want OK, got %q", rec.Body.String())
	}
}

func TestBearerAuth_WhenTokenSetAndEmptyBearerValue_ShouldReturn401(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next should not be called")
	})
	mw := BearerAuth("secret")
	handler := mw(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("empty Bearer value: want 401, got %d", rec.Code)
	}
}

func TestServer_WhenShutdownClosed_ShouldReturnNil(t *testing.T) {
	cfg := &domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}
	srv, err := NewServer(cfg, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	shutdown := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- srv.Run(shutdown) }()
	time.Sleep(30 * time.Millisecond)
	close(shutdown)
	err = <-done
	if err != nil {
		if isListenPermissionErr(err) {
			t.Skip("skipping: cannot bind in this environment (e.g. sandbox)")
		}
		t.Errorf("Run after shutdown: want nil, got %v", err)
	}
}

func TestNewServer_WhenPortZero_ShouldBindRandomPort(t *testing.T) {
	cfg := &domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}
	srv, err := NewServer(cfg, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx.Done()) }()
	time.Sleep(50 * time.Millisecond)
	addr := srv.Addr()
	if addr == "" || addr == ":0" {
		cancel()
		runErr := <-done
		if runErr != nil && isListenPermissionErr(runErr) {
			t.Skip("skipping: cannot bind in this environment (e.g. sandbox)")
		}
		t.Errorf("expected bound addr, got %q (run err: %v)", addr, runErr)
	} else {
		cancel()
		<-done
	}
}

func TestNewServer_WhenConfigNil_ShouldUseDefaults(t *testing.T) {
	srv, err := NewServer(nil, nil)
	if err != nil {
		t.Fatalf("NewServer(nil, nil): %v", err)
	}
	if srv.cfg == nil || srv.cfg.Port != 8080 {
		t.Errorf("expected default port 8080, got %+v", srv.cfg)
	}
}

func TestNewServer_WhenPortInvalid_ShouldReturnError(t *testing.T) {
	_, err := NewServer(&domain.GatewayConfig{Port: -1, Auth: domain.AuthConfig{}}, nil)
	if err != ErrInvalidPort {
		t.Errorf("port -1: want ErrInvalidPort, got %v", err)
	}
	_, err = NewServer(&domain.GatewayConfig{Port: 70000, Auth: domain.AuthConfig{}}, nil)
	if err != ErrInvalidPort {
		t.Errorf("port 70000: want ErrInvalidPort, got %v", err)
	}
}

func TestRun_WhenListenFails_ShouldReturnError(t *testing.T) {
	cfg := &domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}
	srv, err := NewServer(cfg, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	listenErr := errors.New("listen failed")
	oldListen := netListen
	netListen = func(network, address string) (net.Listener, error) {
		return nil, listenErr
	}
	defer func() { netListen = oldListen }()
	shutdown := make(chan struct{})
	close(shutdown)
	err = srv.Run(shutdown)
	if err != listenErr {
		t.Errorf("Run when Listen fails: want %v, got %v", listenErr, err)
	}
	if got := srv.ListenErr(); got != listenErr {
		t.Errorf("ListenErr after Listen fails: want %v, got %v", listenErr, got)
	}
}

func TestRun_WhenShutdownFails_ShouldReturnError(t *testing.T) {
	cfg := &domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}
	srv, err := NewServer(cfg, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	shutdownErr := errors.New("shutdown failed")
	oldShutdown := serverShutdown
	serverShutdown = func(_ *http.Server, _ context.Context) error {
		return shutdownErr
	}
	defer func() { serverShutdown = oldShutdown }()

	shutdown := make(chan struct{})
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(shutdown) }()
	time.Sleep(30 * time.Millisecond)
	close(shutdown)
	got := <-errCh
	if got != nil && isListenPermissionErr(got) {
		t.Skip("skipping: cannot bind in this environment (e.g. sandbox)")
	}
	if got != shutdownErr {
		t.Errorf("Run when Shutdown fails: want %v, got %v", shutdownErr, got)
	}
}

// TestRun_WhenListenSucceeds_ShouldServeUntilShutdown covers Run() success path using a fake listener (no real bind).
func TestRun_WhenListenSucceeds_ShouldServeUntilShutdown(t *testing.T) {
	cfg := &domain.GatewayConfig{Port: 9999, Auth: domain.AuthConfig{}}
	srv, err := NewServer(cfg, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	fakeAddr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9999}
	fl := &fakeListener{addr: fakeAddr, closed: make(chan struct{})}
	oldListen := netListen
	netListen = func(network, address string) (net.Listener, error) {
		return fl, nil
	}
	defer func() { netListen = oldListen }()

	shutdown := make(chan struct{})
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(shutdown) }()
	time.Sleep(20 * time.Millisecond)
	if got := srv.Addr(); got != fakeAddr.String() {
		t.Errorf("Addr(): want %s, got %s", fakeAddr.String(), got)
	}
	close(shutdown)
	err = <-errCh
	if err != nil {
		t.Errorf("Run after shutdown: want nil, got %v", err)
	}
}

// TestRun_WhenShutdownReturnsError_ShouldReturnError covers Run() returning serverShutdown error.
func TestRun_WhenShutdownReturnsError_ShouldReturnError(t *testing.T) {
	cfg := &domain.GatewayConfig{Port: 9999, Auth: domain.AuthConfig{}}
	srv, err := NewServer(cfg, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	fl := &fakeListener{addr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9999}, closed: make(chan struct{})}
	oldListen := netListen
	netListen = func(network, address string) (net.Listener, error) { return fl, nil }
	defer func() { netListen = oldListen }()
	shutdownErr := errors.New("shutdown failed")
	oldShutdown := serverShutdown
	serverShutdown = func(_ *http.Server, _ context.Context) error { return shutdownErr }
	defer func() { serverShutdown = oldShutdown }()

	shutdown := make(chan struct{})
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(shutdown) }()
	time.Sleep(20 * time.Millisecond)
	close(shutdown)
	got := <-errCh
	if got != shutdownErr {
		t.Errorf("Run when Shutdown returns error: want %v, got %v", shutdownErr, got)
	}
}
