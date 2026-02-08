package gateway

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"ironclaw/internal/domain"
)

// ErrInvalidPort is returned when gateway port is not in 0..65535.
var ErrInvalidPort = errors.New("gateway port must be 0-65535")

// Server is an HTTP server that optionally enforces Bearer token auth.
type Server struct {
	cfg       *domain.GatewayConfig
	server    *http.Server
	addr      string
	addrMu    sync.RWMutex
	listenErr error
	listenErrMu sync.Mutex
	listener  net.Listener
}

// NewServer builds a gateway server from config. Port 0 means pick a random port.
// If brain is non-nil, chat messages on /ws use brain.Generate; otherwise replies are echoed.
// Returns ErrInvalidPort if port is not in 0..65535.
func NewServer(cfg *domain.GatewayConfig, brain ChatBrain) (*Server, error) {
	if cfg == nil {
		cfg = &domain.GatewayConfig{Port: 8080, Auth: domain.AuthConfig{}}
	}
	if cfg.Port < 0 || cfg.Port > 65535 {
		return nil, ErrInvalidPort
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) { HandleWS(w, r, brain) })
	handler := BearerAuth(cfg.Auth.AuthToken)(mux)
	s := &Server{
		cfg: cfg,
		server: &http.Server{
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
	return s, nil
}

// Addr returns the bound address (e.g. "127.0.0.1:8080") after Run has started. Empty before Run.
func (s *Server) Addr() string {
	s.addrMu.RLock()
	defer s.addrMu.RUnlock()
	return s.addr
}

// ListenErr returns the error from the initial Listen in Run(), if any. Used when Addr() is still empty after Run() has been started.
func (s *Server) ListenErr() error {
	s.listenErrMu.Lock()
	defer s.listenErrMu.Unlock()
	return s.listenErr
}

// Handler returns the HTTP handler used by the server (BearerAuth + root). For testing without binding.
func (s *Server) Handler() http.Handler {
	return s.server.Handler
}

// netListen is the function used to listen; tests may replace it to force Listen errors.
var netListen = func(network, address string) (net.Listener, error) {
	return net.Listen(network, address)
}

// Run listens on the configured port and serves until shutdown is closed. Returns nil when shutdown.
func (s *Server) Run(shutdown <-chan struct{}) error {
	addr := ":" + strconv.Itoa(s.cfg.Port)
	ln, err := netListen("tcp", addr)
	if err != nil {
		s.listenErrMu.Lock()
		s.listenErr = err
		s.listenErrMu.Unlock()
		return err
	}
	s.addrMu.Lock()
	s.listener = ln
	s.addr = ln.Addr().String()
	s.addrMu.Unlock()

	done := make(chan error, 1)
	go func() {
		done <- s.server.Serve(ln)
	}()

	<-shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = serverShutdown(s.server, ctx)
	if err != nil {
		return err
	}
	<-done
	return nil
}

// serverShutdown is the function used to shut down the server; tests may replace it.
var serverShutdown = func(srv *http.Server, ctx context.Context) error {
	return srv.Shutdown(ctx)
}
