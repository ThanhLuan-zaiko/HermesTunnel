package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"hermes-tunnel/internal/protocol"
	"hermes-tunnel/internal/routing"
)

type Config struct {
	PublicAddr   string
	ControlAddr  string
	Token        string
	MaxBodyBytes int64
	Logf         func(format string, args ...any)
}

type Server struct {
	cfg      Config
	registry *registry
}

func New(cfg Config) (*Server, error) {
	if cfg.PublicAddr == "" {
		cfg.PublicAddr = ":8080"
	}

	if cfg.ControlAddr == "" {
		cfg.ControlAddr = ":8081"
	}

	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = protocol.DefaultMaxBodyBytes
	}

	return &Server{
		cfg:      cfg,
		registry: newRegistry(),
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	controlListener, err := net.Listen("tcp", s.cfg.ControlAddr)
	if err != nil {
		return fmt.Errorf("listen control address %s: %w", s.cfg.ControlAddr, err)
	}
	defer controlListener.Close()

	httpServer := &http.Server{
		Addr:              s.cfg.PublicAddr,
		Handler:           s,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 2)

	go func() {
		s.logf("control server listening on %s", s.cfg.ControlAddr)
		errCh <- s.acceptControl(ctx, controlListener)
	}()

	go func() {
		s.logf("public server listening on %s", s.cfg.PublicAddr)
		err := httpServer.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			errCh <- nil
			return
		}
		errCh <- err
	}()

	go func() {
		<-ctx.Done()
		_ = controlListener.Close()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	if err := <-errCh; err != nil {
		_ = controlListener.Close()
		_ = httpServer.Close()
		return err
	}

	return nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/__hermes/health" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
		return
	}

	name, targetPath, ok := routing.SplitPublicRoute(r)
	if !ok {
		http.Error(w, "Hermes Tunnel route not found. Use /{tunnel-name}/path or a tunnel subdomain.", http.StatusNotFound)
		return
	}

	session, ok := s.registry.get(name)
	if !ok {
		http.Error(w, "Hermes Tunnel is not connected: "+name, http.StatusNotFound)
		return
	}

	body, err := protocol.ReadLimited(r.Body, s.cfg.MaxBodyBytes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	}

	response, err := session.roundTrip(r.Context(), protocol.RequestPayload{
		Method:   r.Method,
		Path:     targetPath,
		RawQuery: r.URL.RawQuery,
		Header:   protocol.CloneHeader(r.Header),
		Body:     body,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	protocol.CopyHeader(w.Header(), response.Header)
	statusCode := response.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusBadGateway
	}

	w.WriteHeader(statusCode)
	_, _ = w.Write(response.Body)
}

func (s *Server) acceptControl(ctx context.Context, listener net.Listener) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}

		go s.handleControlConn(conn)
	}
}

func (s *Server) handleControlConn(conn net.Conn) {
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var register protocol.Message
	if err := decoder.Decode(&register); err != nil {
		_ = conn.Close()
		return
	}

	if register.Type != protocol.TypeRegister {
		_ = encoder.Encode(protocol.Message{Type: protocol.TypeError, Error: "first message must be register"})
		_ = conn.Close()
		return
	}

	register.Name = routing.NormalizeName(register.Name)
	if err := routing.ValidateTunnelName(register.Name); err != nil {
		_ = encoder.Encode(protocol.Message{Type: protocol.TypeError, Error: err.Error()})
		_ = conn.Close()
		return
	}

	if s.cfg.Token != "" && register.Token != s.cfg.Token {
		_ = encoder.Encode(protocol.Message{Type: protocol.TypeError, Error: "invalid token"})
		_ = conn.Close()
		return
	}

	session := newSession(register.Name, conn, encoder)
	if old := s.registry.set(register.Name, session); old != nil {
		old.close()
	}
	defer s.registry.remove(register.Name, session)

	if err := session.send(protocol.Message{Type: protocol.TypeRegistered}); err != nil {
		session.close()
		return
	}

	s.logf("registered tunnel %q from %s", register.Name, conn.RemoteAddr().String())
	session.readLoop(decoder)
	s.logf("disconnected tunnel %q", register.Name)
}

func (s *Server) logf(format string, args ...any) {
	if s.cfg.Logf != nil {
		s.cfg.Logf(format, args...)
	}
}

type registry struct {
	mu       sync.RWMutex
	sessions map[string]*session
}

func newRegistry() *registry {
	return &registry{sessions: make(map[string]*session)}
}

func (r *registry) get(name string) (*session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session, ok := r.sessions[name]
	return session, ok
}

func (r *registry) set(name string, session *session) *session {
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.sessions[name]
	r.sessions[name] = session
	return old
}

func (r *registry) remove(name string, session *session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sessions[name] == session {
		delete(r.sessions, name)
	}
}

type session struct {
	name    string
	conn    net.Conn
	encoder *json.Encoder

	sendMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[string]chan protocol.Message
	counter   atomic.Uint64

	closeOnce sync.Once
	closed    chan struct{}
}

var errSessionClosed = errors.New("tunnel session is closed")

func newSession(name string, conn net.Conn, encoder *json.Encoder) *session {
	return &session{
		name:    name,
		conn:    conn,
		encoder: encoder,
		pending: make(map[string]chan protocol.Message),
		closed:  make(chan struct{}),
	}
}

func (s *session) roundTrip(ctx context.Context, payload protocol.RequestPayload) (protocol.ResponsePayload, error) {
	id := s.nextRequestID()
	responseCh := make(chan protocol.Message, 1)

	s.pendingMu.Lock()
	s.pending[id] = responseCh
	s.pendingMu.Unlock()

	if err := s.send(protocol.Message{Type: protocol.TypeRequest, ID: id, Request: &payload}); err != nil {
		s.removePending(id)
		return protocol.ResponsePayload{}, err
	}

	select {
	case msg := <-responseCh:
		if msg.Type == protocol.TypeError {
			return protocol.ResponsePayload{}, errors.New(msg.Error)
		}
		if msg.Response == nil {
			return protocol.ResponsePayload{}, errors.New("empty tunnel response")
		}
		return *msg.Response, nil
	case <-ctx.Done():
		s.removePending(id)
		return protocol.ResponsePayload{}, ctx.Err()
	case <-s.closed:
		s.removePending(id)
		return protocol.ResponsePayload{}, errSessionClosed
	}
}

func (s *session) nextRequestID() string {
	value := s.counter.Add(1)
	return strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.FormatUint(value, 36)
}

func (s *session) send(msg protocol.Message) error {
	select {
	case <-s.closed:
		return errSessionClosed
	default:
	}

	s.sendMu.Lock()
	err := s.encoder.Encode(msg)
	s.sendMu.Unlock()
	if err != nil {
		s.close()
		return err
	}

	return nil
}

func (s *session) readLoop(decoder *json.Decoder) {
	defer s.close()

	for {
		var msg protocol.Message
		if err := decoder.Decode(&msg); err != nil {
			return
		}

		if msg.Type != protocol.TypeResponse && msg.Type != protocol.TypeError {
			continue
		}

		s.deliver(msg)
	}
}

func (s *session) deliver(msg protocol.Message) {
	s.pendingMu.Lock()
	ch := s.pending[msg.ID]
	delete(s.pending, msg.ID)
	s.pendingMu.Unlock()

	if ch != nil {
		ch <- msg
	}
}

func (s *session) removePending(id string) {
	s.pendingMu.Lock()
	delete(s.pending, id)
	s.pendingMu.Unlock()
}

func (s *session) close() {
	s.closeOnce.Do(func() {
		close(s.closed)
		_ = s.conn.Close()
	})
}
