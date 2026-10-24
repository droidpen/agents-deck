// Package platform contains the desktop integration shared by the CLI and daemon.
package platform

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"
)

// WSL reports whether this process runs inside Windows Subsystem for Linux.
func WSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != "" {
		return true
	}
	b, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	return strings.Contains(strings.ToLower(string(b)), "microsoft")
}

// Shell returns a POSIX login shell. Agent commands use POSIX quoting, so fish
// and other non-POSIX shells cannot be used for this wrapper.
func Shell() string {
	if sh := os.Getenv("SHELL"); filepath.IsAbs(sh) {
		switch filepath.Base(sh) {
		case "bash", "zsh", "sh", "dash", "ksh":
			if st, err := os.Stat(sh); err == nil && st.Mode().IsRegular() && st.Mode()&0o111 != 0 {
				return sh
			}
		}
	}
	if runtime.GOOS == "darwin" {
		return "/bin/zsh"
	}
	if _, err := os.Stat("/bin/bash"); err == nil {
		return "/bin/bash"
	}
	return "/bin/sh"
}

// OpenURL opens a link on the host desktop. EncodedCommand avoids cmd.exe's
// metacharacter expansion and preserves Unicode and URL query strings on WSL.
func OpenURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "http", "https", "file", "codex":
	default:
		return errors.New("unsupported URL scheme")
	}
	if runtime.GOOS == "darwin" {
		return exec.Command("/usr/bin/open", raw).Run()
	}
	if WSL() {
		return exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", openURLCommand(raw)).Run()
	}
	return exec.Command("xdg-open", raw).Run()
}

func openURLCommand(raw string) string {
	script := "$ErrorActionPreference = 'Stop'; Start-Process -FilePath '" + strings.ReplaceAll(raw, "'", "''") + "'"
	u := utf16.Encode([]rune(script))
	b := make([]byte, len(u)*2)
	for i, c := range u {
		binary.LittleEndian.PutUint16(b[i*2:], c)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// Terminal opens a fresh Windows Terminal tab in the same WSL distribution.
// It never reconstructs a shell command from the agent's title or prompt.
func Terminal(argv ...string) error {
	if !WSL() {
		return errors.New("opening a terminal this way requires WSL2")
	}
	distro := os.Getenv("WSL_DISTRO_NAME")
	if distro == "" {
		return errors.New("WSL_DISTRO_NAME is missing; restart the daemon from WSL")
	}
	for _, a := range append([]string{distro}, argv...) {
		if strings.ContainsAny(a, ";\r\n\x00") {
			return errors.New("Windows Terminal arguments cannot contain semicolons or control characters; attach from WSL instead")
		}
	}
	args := []string{"-w", "0", "new-tab", "--", "wsl.exe", "--distribution", distro, "--exec"}
	args = append(args, argv...)
	if out, err := exec.Command("wt.exe", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("Windows Terminal: %w: %s (install Windows Terminal and enable WSL interop)", err, strings.TrimSpace(string(out)))
	}
	return nil
}
