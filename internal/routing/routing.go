package routing

import (
	"errors"
	"net"
	"net/http"
	"regexp"
	"strings"
)

var tunnelNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
var domainLabelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

type Router struct {
	BaseDomain string
}

func NewRouter(baseDomain string) Router {
	return Router{BaseDomain: NormalizeBaseDomain(baseDomain)}
}

func NormalizeName(name string) string {
	return strings.ToLower(name)
}

func NormalizeBaseDomain(domain string) string {
	domain = strings.TrimSpace(strings.ToLower(domain))
	domain = strings.TrimSuffix(domain, ".")
	domain = strings.TrimPrefix(domain, "*.")
	return domain
}

func ValidateBaseDomain(domain string) error {
	domain = NormalizeBaseDomain(domain)
	if domain == "" {
		return nil
	}

	if strings.ContainsAny(domain, "/:") {
		return errors.New("base domain must be a hostname, for example tunnel.example.com")
	}

	if net.ParseIP(domain) != nil {
		return errors.New("base domain must not be an IP address")
	}

	if len(domain) > 253 {
		return errors.New("base domain is too long")
	}

	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return errors.New("base domain must include at least two labels")
	}

	for _, label := range labels {
		if !domainLabelPattern.MatchString(label) {
			return errors.New("base domain contains an invalid DNS label")
		}
	}

	return nil
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
	return NewRouter("").SplitPublicRoute(r)
}

func (rt Router) SplitPublicRoute(r *http.Request) (string, string, bool) {
	if rt.BaseDomain != "" {
		if name, ok := routeNameFromBaseDomain(r.Host, rt.BaseDomain); ok {
			return name, EnsureLeadingSlash(r.URL.Path), true
		}

		if isLocalHost(r.Host) {
			return RouteNameFromPath(r.URL.Path)
		}

		return "", "/", false
	}

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

func routeNameFromBaseDomain(hostport, baseDomain string) (string, bool) {
	host := stripPort(strings.ToLower(hostport))
	baseDomain = NormalizeBaseDomain(baseDomain)
	if host == "" || host == baseDomain {
		return "", false
	}

	suffix := "." + baseDomain
	if !strings.HasSuffix(host, suffix) {
		return "", false
	}

	name := strings.TrimSuffix(host, suffix)
	if strings.Contains(name, ".") || ValidateTunnelName(name) != nil {
		return "", false
	}

	return name, true
}

func routeNameFromHost(hostport string) (string, bool) {
	host := stripPort(strings.ToLower(hostport))
	if isLocalHost(host) {
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

func isLocalHost(hostport string) bool {
	host := stripPort(strings.ToLower(hostport))
	return host == "" || host == "localhost" || net.ParseIP(host) != nil
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
