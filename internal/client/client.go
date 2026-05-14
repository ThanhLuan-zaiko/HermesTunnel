package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"hermes-tunnel/internal/protocol"
	"hermes-tunnel/internal/routing"
)

type Config struct {
	ServerAddr   string
	Name         string
	LocalURL     string
	Token        string
	MaxBodyBytes int64
	Logf         func(format string, args ...any)
}

type Client struct {
	cfg              Config
	localURL         *url.URL
	httpClient       *http.Client
	maxResponseBytes int64
}

func New(cfg Config) (*Client, error) {
	if cfg.ServerAddr == "" {
		cfg.ServerAddr = "127.0.0.1:8081"
	}

	cfg.Name = routing.NormalizeName(cfg.Name)
	if err := routing.ValidateTunnelName(cfg.Name); err != nil {
		return nil, err
	}

	if cfg.LocalURL == "" {
		cfg.LocalURL = "http://localhost:3000"
	}

	localURL, err := url.Parse(cfg.LocalURL)
	if err != nil {
		return nil, fmt.Errorf("parse local URL: %w", err)
	}

	if localURL.Scheme != "http" && localURL.Scheme != "https" {
		return nil, errors.New("local URL must use http or https")
	}

	if localURL.Host == "" {
		return nil, errors.New("local URL must include a host")
	}

	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = protocol.DefaultMaxBodyBytes
	}

	return &Client{
		cfg:      cfg,
		localURL: localURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		maxResponseBytes: cfg.MaxBodyBytes,
	}, nil
}

func (c *Client) Run(ctx context.Context) error {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", c.cfg.ServerAddr)
	if err != nil {
		return fmt.Errorf("connect to Hermes server %s: %w", c.cfg.ServerAddr, err)
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)
	var sendMu sync.Mutex
	send := func(msg protocol.Message) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		return encoder.Encode(msg)
	}

	if err := send(protocol.Message{
		Type:  protocol.TypeRegister,
		Name:  c.cfg.Name,
		Token: c.cfg.Token,
	}); err != nil {
		return fmt.Errorf("register tunnel: %w", err)
	}

	var registered protocol.Message
	if err := decoder.Decode(&registered); err != nil {
		return fmt.Errorf("read registration response: %w", err)
	}

	if registered.Type == protocol.TypeError {
		return errors.New(registered.Error)
	}

	if registered.Type != protocol.TypeRegistered {
		return fmt.Errorf("unexpected registration response %q", registered.Type)
	}

	c.logf("connected tunnel %q to %s through %s", c.cfg.Name, c.localURL.String(), c.cfg.ServerAddr)

	for {
		var msg protocol.Message
		if err := decoder.Decode(&msg); err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read server message: %w", err)
		}

		if msg.Type != protocol.TypeRequest || msg.Request == nil {
			c.logf("ignored unexpected server message %q", msg.Type)
			continue
		}

		go c.handleRequest(ctx, msg, send)
	}
}

func (c *Client) handleRequest(ctx context.Context, msg protocol.Message, send func(protocol.Message) error) {
	response, err := c.forward(ctx, *msg.Request)
	if err != nil {
		if sendErr := send(protocol.Message{Type: protocol.TypeError, ID: msg.ID, Error: err.Error()}); sendErr != nil {
			c.logf("send error response: %v", sendErr)
		}
		return
	}

	if err := send(protocol.Message{Type: protocol.TypeResponse, ID: msg.ID, Response: &response}); err != nil {
		c.logf("send response: %v", err)
	}
}

func (c *Client) forward(ctx context.Context, payload protocol.RequestPayload) (protocol.ResponsePayload, error) {
	target := *c.localURL
	target.Path = joinURLPath(c.localURL.Path, payload.Path)
	target.RawQuery = payload.RawQuery

	req, err := http.NewRequestWithContext(ctx, payload.Method, target.String(), bytes.NewReader(payload.Body))
	if err != nil {
		return protocol.ResponsePayload{}, fmt.Errorf("create local request: %w", err)
	}

	req.Header = protocol.CloneHeader(payload.Header)
	protocol.RemoveHopByHopHeaders(req.Header)
	req.Host = c.localURL.Host

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return protocol.ResponsePayload{}, fmt.Errorf("forward to local service: %w", err)
	}
	defer resp.Body.Close()

	body, err := protocol.ReadLimited(resp.Body, c.maxResponseBytes)
	if err != nil {
		return protocol.ResponsePayload{}, err
	}

	return protocol.ResponsePayload{
		StatusCode: resp.StatusCode,
		Header:     protocol.CloneHeader(resp.Header),
		Body:       body,
	}, nil
}

func (c *Client) logf(format string, args ...any) {
	if c.cfg.Logf != nil {
		c.cfg.Logf(format, args...)
	}
}

func joinURLPath(basePath, requestPath string) string {
	basePath = routing.EnsureLeadingSlash(basePath)
	requestPath = routing.EnsureLeadingSlash(requestPath)

	if basePath == "/" {
		return requestPath
	}

	if requestPath == "/" {
		return basePath
	}

	return strings.TrimRight(basePath, "/") + "/" + strings.TrimLeft(requestPath, "/")
}
