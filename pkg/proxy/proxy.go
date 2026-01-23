package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"github.com/sciffer/agentbox/internal/logger"
	"github.com/sciffer/agentbox/pkg/k8s"
	"github.com/sciffer/agentbox/pkg/models"
)

// NewUpgrader creates a WebSocket upgrader with configurable origin checking
func NewUpgrader(allowedOrigins []string) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  4096,  // Increased for better performance
		WriteBufferSize: 4096, // Increased for better performance
		CheckOrigin: func(r *http.Request) bool {
			// If no origins specified, allow all (development mode)
			if len(allowedOrigins) == 0 {
				return true
			}
			
			origin := r.Header.Get("Origin")
			if origin == "" {
				return false
			}
			
			// Check against allowed origins
			for _, allowed := range allowedOrigins {
				if origin == allowed {
					return true
				}
			}
			
			return false
		},
		// Enable compression for better performance
		EnableCompression: true,
	}
}

var upgrader = NewUpgrader(nil) // Default: allow all origins (can be overridden)

// Proxy handles WebSocket connections to pod shells
type Proxy struct {
	k8sClient     k8s.ClientInterface
	logger        *logger.Logger
	sessions      map[string]*Session
	mu            sync.RWMutex
	upgrader      websocket.Upgrader
	maxSessions   int
	sessionCount  int
}

// Session represents an active WebSocket session
type Session struct {
	ID        string
	Namespace string
	PodName   string
	Conn      *websocket.Conn
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	cancel    context.CancelFunc
	closed    bool
	mu        sync.Mutex
}

// NewProxy creates a new WebSocket proxy
func NewProxy(k8sClient k8s.ClientInterface, log *logger.Logger) *Proxy {
	return &Proxy{
		k8sClient:   k8sClient,
		logger:      log,
		sessions:    make(map[string]*Session),
		upgrader:    NewUpgrader(nil), // Default: allow all origins
		maxSessions: 100,               // Limit concurrent sessions
	}
}

// NewProxyWithConfig creates a new WebSocket proxy with custom configuration
func NewProxyWithConfig(k8sClient k8s.ClientInterface, log *logger.Logger, allowedOrigins []string, maxSessions int) *Proxy {
	return &Proxy{
		k8sClient:   k8sClient,
		logger:      log,
		sessions:    make(map[string]*Session),
		upgrader:    NewUpgrader(allowedOrigins),
		maxSessions: maxSessions,
	}
}

// HandleWebSocket handles WebSocket upgrade and connection
func (p *Proxy) HandleWebSocket(w http.ResponseWriter, r *http.Request, namespace, podName string) error {
	// Check session limit
	p.mu.RLock()
	sessionCount := len(p.sessions)
	p.mu.RUnlock()
	
	if sessionCount >= p.maxSessions {
		return fmt.Errorf("maximum session limit reached (%d)", p.maxSessions)
	}
	
	// Upgrade HTTP connection to WebSocket
	conn, err := p.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return fmt.Errorf("failed to upgrade connection: %w", err)
	}

	sessionID := fmt.Sprintf("%s-%s-%d", namespace, podName, time.Now().Unix())
	
	ctx, cancel := context.WithCancel(context.Background())
	
	session := &Session{
		ID:        sessionID,
		Namespace: namespace,
		PodName:   podName,
		Conn:      conn,
		cancel:    cancel,
	}

	// Store session
	p.mu.Lock()
	p.sessions[sessionID] = session
	p.mu.Unlock()

	p.logger.Info("websocket session started",
		zap.String("session_id", sessionID),
		zap.String("namespace", namespace),
		zap.String("pod", podName),
	)

	// Start handling session
	go p.handleSession(ctx, session)

	return nil
}

// handleSession manages the WebSocket session lifecycle
func (p *Proxy) handleSession(ctx context.Context, session *Session) {
	defer p.cleanup(session)

	// Create pipes for I/O
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	session.stdin = stdinWriter
	session.stdout = stdoutReader
	session.stderr = stderrReader

	// Start pod exec in background
	go func() {
		err := p.k8sClient.ExecInPod(
			ctx,
			session.Namespace,
			session.PodName,
			[]string{"/bin/sh"},
			stdinReader,
			stdoutWriter,
			stderrWriter,
		)
		if err != nil {
			p.logger.Error("pod exec failed",
				zap.String("session_id", session.ID),
				zap.Error(err),
			)
		}
		session.Close()
	}()

	// Handle stdout
	go p.streamOutput(session, stdoutReader, "stdout")

	// Handle stderr
	go p.streamOutput(session, stderrReader, "stderr")

	// Handle stdin (WebSocket messages)
	p.handleInput(session)
}

// handleInput reads from WebSocket and writes to pod stdin
func (p *Proxy) handleInput(session *Session) {
	for {
		var msg models.WebSocketMessage
		err := session.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				p.logger.Info("websocket closed normally", zap.String("session_id", session.ID))
			} else {
				p.logger.Error("failed to read websocket message",
					zap.String("session_id", session.ID),
					zap.Error(err),
				)
			}
			session.Close()
			return
		}

		if msg.Type == "stdin" {
			_, err := session.stdin.Write([]byte(msg.Data))
			if err != nil {
				p.logger.Error("failed to write to stdin",
					zap.String("session_id", session.ID),
					zap.Error(err),
				)
				session.Close()
				return
			}
		}
	}
}

// streamOutput reads from pod output and writes to WebSocket
func (p *Proxy) streamOutput(session *Session, reader io.Reader, streamType string) {
	// Use larger buffer for better performance
	buf := make([]byte, 16384) // 16KB buffer
	now := time.Now()
	
	for {
		n, err := reader.Read(buf)
		if err != nil {
			if err != io.EOF {
				p.logger.Error("failed to read from pod",
					zap.String("session_id", session.ID),
					zap.String("stream", streamType),
					zap.Error(err),
				)
			}
			return
		}

		if n > 0 {
			// Reuse timestamp for batch operations
			msg := models.WebSocketMessage{
				Type:      streamType,
				Data:      string(buf[:n]),
				Timestamp: now,
			}

			session.mu.Lock()
			closed := session.closed
			if !closed {
				err = session.Conn.WriteJSON(msg)
				if err != nil {
					p.logger.Error("failed to write to websocket",
						zap.String("session_id", session.ID),
						zap.Error(err),
					)
					session.mu.Unlock()
					session.Close()
					return
				}
			}
			session.mu.Unlock()
			
			if closed {
				return
			}
			
			// Update timestamp for next message
			now = time.Now()
		}
	}
}

// cleanup closes session and removes it from active sessions
func (p *Proxy) cleanup(session *Session) {
	session.Close()

	p.mu.Lock()
	delete(p.sessions, session.ID)
	p.mu.Unlock()

	p.logger.Info("websocket session ended", zap.String("session_id", session.ID))
}

// Close closes a session
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	s.closed = true
	s.cancel()

	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.stdout != nil {
		s.stdout.Close()
	}
	if s.stderr != nil {
		s.stderr.Close()
	}

	// Send close message
	closeMsg := models.WebSocketMessage{
		Type:      "exit",
		Timestamp: time.Now(),
	}
	s.Conn.WriteJSON(closeMsg)
	s.Conn.Close()
}

// GetActiveSessions returns the number of active sessions
func (p *Proxy) GetActiveSessions() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.sessions)
}

// CloseSession closes a specific session by ID
func (p *Proxy) CloseSession(sessionID string) error {
	p.mu.RLock()
	session, exists := p.sessions[sessionID]
	p.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found")
	}

	session.Close()
	return nil
}
