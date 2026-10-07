package audit

import "testing"

func TestActiveDirective(t *testing.T) {
	cfg := "# PasswordAuthentication yes\nPasswordAuthentication no\nPermitRootLogin yes\n"
	if activeDirective(cfg, "passwordauthentication", "yes") {
		t.Fatal("commented/disabled password auth incorrectly detected")
	}
	if !activeDirective(cfg, "permitrootlogin", "yes") {
		t.Fatal("active root directive not detected")
	}
}
