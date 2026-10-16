package ports

import (
	"reflect"
	"testing"

	"github.com/songji-tian/agentdock/internal/config"
)

func TestAllocate(t *testing.T) {
	services := []config.Service{
		{Name: "nginx", Ports: []string{"${WEB_PORT:-80}:80", "${WEB_SSL_PORT:-443}:443"}},
		{Name: "web", Ports: []string{}},
		{Name: "api", Ports: []string{"${API_PORT:-8000}:8000"}},
		{Name: "redis", Ports: []string{"6379:6379"}},
	}
	got := Allocate(services, 41000)
	want := map[string][]string{
		"api":   {"41000:8000"},
		"nginx": {"41001:80", "41002:443"},
		"redis": {"41003:6379"},
		"web":   {},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Allocate:\n got=%v\nwant=%v", got, want)
	}
}

func TestAllocateContainerOnly(t *testing.T) {
	services := []config.Service{
		{Name: "solo", Ports: []string{"3000"}}, // container-only, no host
		{Name: "pub", Ports: []string{"8000:8000"}},
	}
	got := Allocate(services, 50000)
	want := map[string][]string{
		"pub":  {"50000:8000"},
		"solo": {"3000"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Allocate:\n got=%v\nwant=%v", got, want)
	}
}

func TestSplitPort(t *testing.T) {
	cases := []struct {
		spec      string
		host      string
		container string
		published bool
	}{
		{"${API_PORT:-8000}:8000", "${API_PORT:-8000}", "8000", true},
		{"8000:8000", "8000", "8000", true},
		{"127.0.0.1:8000:8000", "127.0.0.1:8000", "8000", true},
		{"8000", "", "8000", false},
		{":8000", "", "8000", false},
	}
	for _, c := range cases {
		h, cn, pub := splitPort(c.spec)
		if h != c.host || cn != c.container || pub != c.published {
			t.Errorf("splitPort(%q) = (%q,%q,%v), want (%q,%q,%v)",
				c.spec, h, cn, pub, c.host, c.container, c.published)
		}
	}
}
