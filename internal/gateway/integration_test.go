package gateway

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hermes-tunnel/internal/client"
)

func TestServerClientRoundTrip(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read local request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/hello" {
			t.Errorf("expected /hello, got %s", r.URL.Path)
		}
		if r.URL.RawQuery != "x=1" {
			t.Errorf("expected x=1, got %s", r.URL.RawQuery)
		}

		w.Header().Set("X-Hermes-Test", "ok")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("local:" + string(body)))
	}))
	defer local.Close()

	publicAddr := freeTCPAddr(t)
	controlAddr := freeTCPAddr(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server, err := New(Config{
		PublicAddr:  publicAddr,
		ControlAddr: controlAddr,
		BaseDomain:  "tunnel.example.com",
		Token:       "secret",
		Logf:        t.Logf,
	})
	if err != nil {
		t.Fatal(err)
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Run(ctx)
	}()

	waitForHealth(t, "http://"+publicAddr+"/__hermes/health")

	tunnelClient, err := client.New(client.Config{
		ServerAddr: controlAddr,
		Name:       "app",
		LocalURL:   local.URL,
		Token:      "secret",
		Logf:       t.Logf,
	})
	if err != nil {
		t.Fatal(err)
	}

	clientErr := make(chan error, 1)
	go func() {
		clientErr <- tunnelClient.Run(ctx)
	}()

	resp := waitForPost(t, "http://"+publicAddr+"/hello?x=1", "app.tunnel.example.com", "ping")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Hermes-Test") != "ok" {
		t.Fatalf("expected response header from local service")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "local:ping" {
		t.Fatalf("expected local:ping, got %q", string(body))
	}

	cancel()
	expectCleanStop(t, serverErr)
	expectCleanStop(t, clientErr)
}

func freeTCPAddr(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	return listener.Addr().String()
}

func waitForHealth(t *testing.T, url string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", url)
}

func waitForPost(t *testing.T, url string, host string, body string) *http.Response {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Host = host
		req.Header.Set("Content-Type", "text/plain")

		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusBadGateway {
			return resp
		}

		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for tunnel route, last error: %v", lastErr)
	return nil
}

func expectCleanStop(t *testing.T, errCh <-chan error) {
	t.Helper()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for shutdown")
	}
}
