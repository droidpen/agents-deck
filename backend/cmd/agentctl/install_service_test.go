package main

import (
	"strings"
	"testing"
)

func TestLinuxUnitEscapesPaths(t *testing.T) {
	u := linuxUnit(`/tmp/a b%h$USER/agentctl`, `/tmp/home`, `/tmp/private`, "")
	for _, want := range []string{`ExecStart="/tmp/a b%%h$$USER/agentctl" serve`, `Environment="AGENTSTERM_HOME=/tmp/private"`, "UMask=0077", "Restart=on-failure"} {
		if !strings.Contains(u, want) {
			t.Errorf("unit missing %q:\n%s", want, u)
		}
	}
}
