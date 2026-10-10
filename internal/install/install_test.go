package install

import "testing"

func TestPresetCatalog(t *testing.T) {
	ids := map[string]bool{}
	for _, p := range Catalog {
		if ids[p.ID] || p.ID == "" || p.Name == "" || len(p.Packages) == 0 {
			t.Fatalf("invalid preset: %+v", p)
		}
		ids[p.ID] = true
		if _, ok := Find(p.ID); !ok {
			t.Fatalf("preset %s missing", p.ID)
		}
		for pm, packages := range p.Packages {
			if pm == "" || len(packages) == 0 {
				t.Fatalf("empty package mapping: %s %s", p.ID, pm)
			}
		}
	}
	redis, _ := Find("redis")
	if services := serviceNames(redis, "apt-get"); len(services) != 1 || services[0] != "redis-server" {
		t.Fatal(services)
	}
	if services := serviceNames(redis, "apk"); len(services) != 1 || services[0] != "redis" {
		t.Fatal(services)
	}
}

func TestPanelInput(t *testing.T) {
	for _, domain := range []string{"https://panel.example.com", "bad;include evil", "a\nlisten 443", "-bad.example", "a..com", "bad-.com"} {
		if err := validatePanelInput(domain, "admin@example.com"); err == nil {
			t.Errorf("accepted domain %q", domain)
		}
	}
	if err := validatePanelInput("panel.example.com", "bad\nmail"); err == nil {
		t.Fatal("accepted invalid email")
	}
	if err := validatePanelInput("panel.example.com", "admin@example.com"); err != nil {
		t.Fatal(err)
	}
}
