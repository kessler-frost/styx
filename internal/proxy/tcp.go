package proxy

import (
	"context"
	"io"
	"net"
	"sync"
)

// TCPProxy forwards TCP connections from a listen address to a target address.
type TCPProxy struct {
	listenAddr string
	targetAddr string

	listener net.Listener
	mu       sync.Mutex
	conns    map[net.Conn]struct{}
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewTCPProxy creates a new TCP proxy that forwards from listenAddr to targetAddr.
// Example: NewTCPProxy("0.0.0.0:10080", "192.168.64.4:80")
func NewTCPProxy(listenAddr, targetAddr string) *TCPProxy {
	ctx, cancel := context.WithCancel(context.Background())
	return &TCPProxy{
		listenAddr: listenAddr,
		targetAddr: targetAddr,
		conns:      make(map[net.Conn]struct{}),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start begins listening and forwarding connections.
// This method blocks until the proxy is stopped or an error occurs.
func (p *TCPProxy) Start() error {
	listener, err := net.Listen("tcp", p.listenAddr)
	if err != nil {
		return err
	}
	p.listener = listener

	// Accept connections in a loop
	for {
		conn, err := listener.Accept()
		if err != nil {
			// Check if we're shutting down
			select {
			case <-p.ctx.Done():
				return nil
			default:
				return err
			}
		}

		p.trackConn(conn, true)
		go p.handleConn(conn)
	}
}

// StartAsync starts the proxy in a goroutine and returns immediately.
// Returns an error if the listener cannot be created.
func (p *TCPProxy) StartAsync() error {
	listener, err := net.Listen("tcp", p.listenAddr)
	if err != nil {
		return err
	}
	p.listener = listener

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-p.ctx.Done():
					return
				default:
					continue
				}
			}

			p.trackConn(conn, true)
			go p.handleConn(conn)
		}
	}()

	return nil
}

// Stop closes the listener and all active connections.
func (p *TCPProxy) Stop() {
	p.cancel()

	if p.listener != nil {
		p.listener.Close()
	}

	// Close all tracked connections
	p.mu.Lock()
	for conn := range p.conns {
		conn.Close()
	}
	p.conns = make(map[net.Conn]struct{})
	p.mu.Unlock()
}

// ListenAddr returns the address the proxy is listening on.
func (p *TCPProxy) ListenAddr() string {
	return p.listenAddr
}

// TargetAddr returns the address the proxy forwards to.
func (p *TCPProxy) TargetAddr() string {
	return p.targetAddr
}

func (p *TCPProxy) trackConn(conn net.Conn, add bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if add {
		p.conns[conn] = struct{}{}
	} else {
		delete(p.conns, conn)
	}
}

func (p *TCPProxy) handleConn(src net.Conn) {
	defer src.Close()
	defer p.trackConn(src, false)

	// Dial the target
	dst, err := net.Dial("tcp", p.targetAddr)
	if err != nil {
		return
	}
	defer dst.Close()
	p.trackConn(dst, true)
	defer p.trackConn(dst, false)

	// Copy data in both directions
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(dst, src)
		dst.Close() // Signal EOF to target
	}()

	go func() {
		defer wg.Done()
		io.Copy(src, dst)
		src.Close() // Signal EOF to source
	}()

	wg.Wait()
}
