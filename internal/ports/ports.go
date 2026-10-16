// Package ports assigns host ports to services deterministically.
package ports

import (
	"fmt"
	"sort"
	"strings"

	"github.com/songji-tian/agentdock/internal/config"
)

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
