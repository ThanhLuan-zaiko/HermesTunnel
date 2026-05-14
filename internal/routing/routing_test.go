package routing

import (
	"net/http"
	"testing"
)

func TestRouteNameFromPath(t *testing.T) {
	name, path, ok := RouteNameFromPath("/app/api/users")
	if !ok {
		t.Fatal("expected route")
	}
	if name != "app" {
		t.Fatalf("expected app, got %q", name)
	}
	if path != "/api/users" {
		t.Fatalf("expected /api/users, got %q", path)
	}
}

func TestRouteNameFromLocalhostSubdomain(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://app.localhost:8080/users", nil)
	if err != nil {
		t.Fatal(err)
	}

	name, path, ok := SplitPublicRoute(req)
	if !ok {
		t.Fatal("expected route")
	}
	if name != "app" {
		t.Fatalf("expected app, got %q", name)
	}
	if path != "/users" {
		t.Fatalf("expected /users, got %q", path)
	}
}

func TestValidateTunnelNameRejectsUppercase(t *testing.T) {
	if err := ValidateTunnelName("MyApp"); err == nil {
		t.Fatal("expected uppercase name to be rejected")
	}
}
