package platform

import (
	"encoding/base64"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestBrowserCommandPreservesUnicodeAndQuotes(t *testing.T) {
	raw := "https://example.invalid/?q=it's&text=日本語;$()"
	b, err := base64.StdEncoding.DecodeString(openURLCommand(raw))
	if err != nil {
		t.Fatal(err)
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	want := "$ErrorActionPreference = 'Stop'; Start-Process -FilePath 'https://example.invalid/?q=it''s&text=日本語;$()'"
	if got := string(utf16.Decode(u)); got != want {
		t.Fatalf("browser command = %q, want %q", got, want)
	}
}

func TestOpenURLRejectsCommands(t *testing.T) {
	for _, u := range []string{"javascript:alert(1)", "data:text/html,x", "not-a-url", "https://%zz"} {
		if err := OpenURL(u); err == nil {
			t.Errorf("accepted %q", u)
		}
	}
}

func TestShellIgnoresNonPOSIXShell(t *testing.T) {
	t.Setenv("SHELL", "/usr/bin/fish")
	if got := filepath.Base(Shell()); got != "bash" && got != "zsh" && got != "sh" {
		t.Fatalf("non-POSIX wrapper: %q", got)
	}
}

func TestWSLDesktopInterop(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("WSL detection is Linux-only")
	}
	t.Setenv("WSL_DISTRO_NAME", "Test Distro")
	if !WSL() {
		t.Fatal("WSL distribution not detected")
	}
	dir := t.TempDir()
	capture := filepath.Join(dir, "args")
	t.Setenv("CAPTURE", capture)
	t.Setenv("PATH", dir)
	for _, name := range []string{"wt.exe", "powershell.exe"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$CAPTURE\"\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := Terminal("/tmp/project space/agentctl", "attach", "abc123"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	want := "-w\n0\nnew-tab\n--\nwsl.exe\n--distribution\nTest Distro\n--exec\n/tmp/project space/agentctl\nattach\nabc123\n"
	if string(b) != want {
		t.Fatalf("terminal argv = %q, want %q", b, want)
	}
	if err := Terminal("/tmp/project;other/agentctl"); err == nil {
		t.Fatal("accepted a Windows Terminal command separator")
	}
	if err := OpenURL("http://127.0.0.1:7340/login?code=example"); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(capture)
	if !strings.HasPrefix(string(b), "-NoProfile\n-NonInteractive\n-EncodedCommand\n") {
		t.Fatalf("unsafe browser invocation: %q", b)
	}
}

func TestDaemonLockIsExclusiveAndReleased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.lock")
	unlock, err := Lock(path)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := Lock(path); err == nil {
		other()
		t.Error("second daemon acquired the same lock")
	}
	unlock()
	unlock, err = Lock(path)
	if err != nil {
		t.Fatal("lock was not released:", err)
	}
	unlock()
}
