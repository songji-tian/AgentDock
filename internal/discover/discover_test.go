package discover

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/songji-tian/agentdock/internal/config"
)

func TestDiscover(t *testing.T) {
	res, err := Discover("testdata/compose")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Services) != 6 {
		t.Fatalf("want 6 services, got %d", len(res.Services))
	}
	by := map[string]*config.Service{}
	for i := range res.Services {
		by[res.Services[i].Name] = &res.Services[i]
	}

	db := by["db"]
	if db == nil {
		t.Fatal("db missing")
	}
	if db.ContainerName != "demo-db" {
		t.Errorf("db container_name = %q, want demo-db", db.ContainerName)
	}
	if len(db.Ports) != 1 || db.Ports[0] != "${DB_PORT:-5432}:5432" {
		t.Errorf("db ports = %v", db.Ports)
	}
	if len(db.RequiredEnv) != 1 || db.RequiredEnv[0] != "POSTGRES_PASSWORD" {
		t.Errorf("db required_env = %v", db.RequiredEnv)
	}

	fw := by["feishu-webhook"]
	if fw == nil {
		t.Fatal("feishu-webhook missing")
	}
	if len(fw.Profiles) != 1 || fw.Profiles[0] != "feishu" {
		t.Errorf("feishu-webhook profiles = %v", fw.Profiles)
	}
	if len(fw.Ports) != 1 || fw.Ports[0] != "3001:3001" {
		t.Errorf("feishu-webhook ports = %v", fw.Ports)
	}

	web := by["web"]
	if web == nil {
		t.Fatal("web missing")
	}
	if web.Ports == nil {
		t.Error("web.Ports must be non-nil so JSON emits [] not null")
	}
	if len(web.Ports) != 0 {
		t.Errorf("web should have no exposed ports, got %v", web.Ports)
	}
	if web.Image != "demo-web-custom:latest" {
		t.Errorf("override merge failed: web image = %q", web.Image)
	}

	ng := by["nginx"]
	if ng == nil {
		t.Fatal("nginx missing")
	}
	if len(ng.Ports) != 2 {
		t.Errorf("nginx ports = %v, want 2", ng.Ports)
	}

	api := by["api"]
	if api == nil {
		t.Fatal("api missing")
	}
	if len(api.EnvFiles) != 1 || api.EnvFiles[0] != ".env" {
		t.Errorf("api env_files = %v", api.EnvFiles)
	}

	// total exposed ports across the stack must be 6 (5432,6379,8000,3001,80,443)
	total := 0
	for _, s := range res.Services {
		total += len(s.Ports)
	}
	if total != 6 {
		t.Errorf("total ports = %d, want 6", total)
	}
}

func TestPortMappingAndTags(t *testing.T) {
	dir := t.TempDir()
	compose := `services:
  app:
    container_name: demo-app
    ports:
      - target: 8080
        published: 18080
      - "9090:9090"
    volumes: !override []
`
	if err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte(compose), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Services) != 1 {
		t.Fatalf("want 1 service, got %d", len(res.Services))
	}
	got := res.Services[0].Ports
	if len(got) != 2 || got[0] != "18080:8080" || got[1] != "9090:9090" {
		t.Errorf("ports = %v, want [18080:8080 9090:9090]", got)
	}
}
