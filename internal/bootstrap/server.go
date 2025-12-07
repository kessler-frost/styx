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
	mux.HandleFunc("/bootstrap/consul-client-cert.pem", s.serveConsulClientCert)
	mux.HandleFunc("/bootstrap/consul-client-key.pem", s.serveConsulClientKey)
	mux.HandleFunc("/bootstrap/nomad-ca.pem", s.serveNomadCA)
	mux.HandleFunc("/bootstrap/nomad-client-cert.pem", s.serveNomadClientCert)
	mux.HandleFunc("/bootstrap/nomad-client-key.pem", s.serveNomadClientKey)
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
	s.serveFile(w, filepath.Join(s.certsDir, "consul-agent-ca.pem"), "application/x-pem-file")
}

func (s *Server) serveConsulClientCert(w http.ResponseWriter, r *http.Request) {
	// Serve dc1-client-consul-0.pem (generated on server for clients)
	s.serveFile(w, filepath.Join(s.certsDir, "dc1-client-consul-0.pem"), "application/x-pem-file")
}

func (s *Server) serveConsulClientKey(w http.ResponseWriter, r *http.Request) {
	s.serveFile(w, filepath.Join(s.certsDir, "dc1-client-consul-0-key.pem"), "application/x-pem-file")
}

func (s *Server) serveNomadCA(w http.ResponseWriter, r *http.Request) {
	s.serveFile(w, filepath.Join(s.certsDir, "nomad-agent-ca.pem"), "application/x-pem-file")
}

func (s *Server) serveNomadClientCert(w http.ResponseWriter, r *http.Request) {
	s.serveFile(w, filepath.Join(s.certsDir, "global-client-nomad.pem"), "application/x-pem-file")
}

func (s *Server) serveNomadClientKey(w http.ResponseWriter, r *http.Request) {
	s.serveFile(w, filepath.Join(s.certsDir, "global-client-nomad-key.pem"), "application/x-pem-file")
}

func (s *Server) serveFile(w http.ResponseWriter, path, contentType string) {
	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
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
