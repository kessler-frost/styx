package bootstrap

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	// Port is the bootstrap server port
	Port = 19999
)

// Server serves bootstrap files (CA cert, gossip key) for new clients.
// Only accessible on Tailscale network for security.
type Server struct {
	certsDir   string
	secretsDir string
	server     *http.Server
	listener   net.Listener
}

// NewServer creates a new bootstrap server.
func NewServer(tailscaleIP, certsDir, secretsDir string) (*Server, error) {
	s := &Server{
		certsDir:   certsDir,
		secretsDir: secretsDir,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/bootstrap/consul-ca.pem", s.serveConsulCA)
	mux.HandleFunc("/bootstrap/gossip.key", s.serveGossipKey)
	mux.HandleFunc("/bootstrap/health", s.serveHealth)

	addr := fmt.Sprintf("%s:%d", tailscaleIP, Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.listener = listener
	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return s, nil
}

// Start starts the bootstrap server in a goroutine.
func (s *Server) Start() {
	go s.server.Serve(s.listener)
}

// Stop gracefully stops the bootstrap server.
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// Addr returns the address the server is listening on.
func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

func (s *Server) serveConsulCA(w http.ResponseWriter, r *http.Request) {
	caPath := filepath.Join(s.certsDir, "consul-agent-ca.pem")
	data, err := os.ReadFile(caPath)
	if err != nil {
		http.Error(w, "CA not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Write(data)
}

func (s *Server) serveGossipKey(w http.ResponseWriter, r *http.Request) {
	keyPath := filepath.Join(s.secretsDir, "gossip.key")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		http.Error(w, "Gossip key not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write(data)
}

func (s *Server) serveHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
