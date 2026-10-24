package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/brushknight/agents-deck/backend/internal/model"
	"github.com/brushknight/agents-deck/backend/internal/paths"
	"github.com/brushknight/agents-deck/backend/internal/platform"
)

// installLinux supports a systemd user service. Windows installs use Task
// Scheduler instead, so WSL2 does not need systemd or a lingering user manager.
func installLinux() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if err := paths.Ensure(); err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	unit := linuxUnit(self, home, paths.Data(), os.Getenv("WSL_DISTRO_NAME"))
	path := filepath.Join(dir, plistLabel+".service")
	if err := os.WriteFile(path, []byte(unit), 0o600); err != nil {
		return err
	}
	for _, args := range [][]string{{"--user", "daemon-reload"}, {"--user", "enable", plistLabel+".service"}, {"--user", "restart", plistLabel+".service"}} {
		if out, err := exec.Command("systemctl", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("systemd user service: %w: %s; on Windows use install.ps1, or run `agentctl serve` in a terminal", err, strings.TrimSpace(string(out)))
		}
	}
	fmt.Println("installed", path)
	return nil
}

// systemd expands percent specifiers and $variables in ExecStart; double
// them before quoting paths, and escape Environment values independently.
func unitQuote(s string) string {
	s = strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "%", "%%", "\n", "\\n", "\r", "\\r").Replace(s)
	return "\"" + s + "\""
}

func linuxUnit(self, home, data, distro string) string {
	unit := "[Unit]\nDescription=agents deck\n\n[Service]\nType=simple\nExecStart=" + unitQuote(strings.ReplaceAll(self, "$", "$$")) + " serve\n" +
		"Environment=" + unitQuote("PATH="+home+"/.local/bin:/usr/local/bin:/usr/bin:/bin") + "\n" +
		"Environment=" + unitQuote("AGENTSTERM_HOME="+data) + "\n"
	if distro != "" {
		unit += "Environment=" + unitQuote("WSL_DISTRO_NAME="+distro) + "\n"
		// Desktop integration depends on Windows' inherited interop PATH.
		unit += "Environment=" + unitQuote("PATH="+os.Getenv("PATH")) + "\n"
	}
	return unit + "Restart=on-failure\nRestartSec=3\nUMask=0077\n\n[Install]\nWantedBy=default.target\n"
}

func reopenWSL() error {
	s, err := state()
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	n := 0
	for _, a := range s.Agents {
		if a.Attached || a.External || a.Status == model.Exited {
			continue
		}
		if err := platform.Terminal(self, "attach", a.ID); err != nil {
			return err
		}
		n++
	}
	fmt.Printf("opened %d tab(s) in Windows Terminal\n", n)
	return nil
}
