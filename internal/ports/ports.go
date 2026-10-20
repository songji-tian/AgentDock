// Package ports assigns host ports to services deterministically.
package ports

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/songji-tian/agentdock/internal/config"
)

// CountPublished returns how many host ports the services occupy once
// Allocate runs. Container-only ports (no host side) consume no slot.
func CountPublished(services []config.Service) int {
	n := 0
	for _, s := range services {
		for _, p := range s.Ports {
			if _, _, published := splitPort(p); published {
				n++
			}
		}
	}
	return n
}

// PortFree reports whether a host TCP port is not currently bound. It binds
// the port briefly and releases it, so true is a point-in-time observation
// (a small TOCTOU window remains before Docker actually binds the port).
func PortFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

// BlockFree reports whether n consecutive host ports starting at base are all
// unbound.
func BlockFree(base, n int) bool {
	for i := 0; i < n; i++ {
		if !PortFree(base + i) {
			return false
		}
	}
	return true
}

// Allocate assigns consecutive host ports to each service's published ports,
// processing services in name order (deterministic). Container-only ports
// (no host side) pass through unchanged and consume no slot.
//
// A published port "HOST:CONTAINER" becomes "<slot>:CONTAINER" — the container
// side never changes; only the host side is remapped onto the dock's block.
func Allocate(services []config.Service, portBase int) map[string][]string {
	names := make([]string, 0, len(services))
	byName := map[string]config.Service{}
	for _, s := range services {
		names = append(names, s.Name)
		byName[s.Name] = s
	}
	sort.Strings(names)

	out := map[string][]string{}
	next := portBase
	for _, name := range names {
		svc := byName[name]
		mapped := make([]string, 0, len(svc.Ports))
		for _, p := range svc.Ports {
			_, container, published := splitPort(p)
			if !published {
				mapped = append(mapped, p) // container-only: leave untouched
				continue
			}
			mapped = append(mapped, fmt.Sprintf("%d:%s", next, container))
			next++
		}
		out[name] = mapped
	}
	return out
}

// splitPort splits a "HOST:CONTAINER" port spec on the LAST colon so a host of
// the form "ip:port" (or "${VAR:-port}") stays intact. Returns published=false
// for container-only specs with no host side.
func splitPort(spec string) (host, container string, published bool) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", "", false
	}
	i := strings.LastIndex(spec, ":")
	if i < 0 {
		return "", spec, false // container-only
	}
	host = spec[:i]
	container = spec[i+1:]
	if host == "" {
		return "", container, false // ":8000" = random host, leave untouched
	}
	return host, container, true
}
