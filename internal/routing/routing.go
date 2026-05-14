package routing

import (
	"errors"
	"net"
	"net/http"
	"regexp"
	"strings"
)

var tunnelNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

func NormalizeName(name string) string {
	return strings.ToLower(name)
}

func ValidateTunnelName(name string) error {
	if name == "" {
		return errors.New("tunnel name is required")
	}

	if name != strings.ToLower(name) {
		return errors.New("tunnel name must be lowercase")
	}

	if !tunnelNamePattern.MatchString(name) {
		return errors.New("tunnel name must start with a letter or digit and contain only lowercase letters, digits, or hyphens")
	}

	return nil
}

func SplitPublicRoute(r *http.Request) (string, string, bool) {
	if name, ok := routeNameFromHost(r.Host); ok {
		return name, EnsureLeadingSlash(r.URL.Path), true
	}

	return RouteNameFromPath(r.URL.Path)
}

func RouteNameFromPath(path string) (string, string, bool) {
	path = EnsureLeadingSlash(path)
	trimmed := strings.TrimPrefix(path, "/")
	if trimmed == "" {
		return "", "/", false
	}

	name, rest, _ := strings.Cut(trimmed, "/")
	name = NormalizeName(name)
	if ValidateTunnelName(name) != nil {
		return "", "/", false
	}

	if rest == "" {
		return name, "/", true
	}

	return name, "/" + rest, true
}

func EnsureLeadingSlash(path string) string {
	if path == "" {
		return "/"
	}

	if strings.HasPrefix(path, "/") {
		return path
	}

	return "/" + path
}

func routeNameFromHost(hostport string) (string, bool) {
	host := stripPort(strings.ToLower(hostport))
	if host == "" || host == "localhost" || net.ParseIP(host) != nil {
		return "", false
	}

	if strings.HasSuffix(host, ".localhost") {
		name := strings.TrimSuffix(host, ".localhost")
		if !strings.Contains(name, ".") && ValidateTunnelName(name) == nil {
			return name, true
		}
		return "", false
	}

	parts := strings.Split(host, ".")
	if len(parts) < 3 {
		return "", false
	}

	name := parts[0]
	if ValidateTunnelName(name) != nil {
		return "", false
	}

	return name, true
}

func stripPort(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err == nil {
		return host
	}

	if i := strings.LastIndex(hostport, ":"); i > -1 && !strings.Contains(hostport[i+1:], "]") {
		return strings.Trim(hostport[:i], "[]")
	}

	return strings.Trim(hostport, "[]")
}
