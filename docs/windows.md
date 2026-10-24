# Windows (WSL2)

The deck runs inside WSL2, with its dashboard in your Windows browser. Start
agents in WSL and answer their prompts from the dashboard, attach from a WSL
terminal, or use Focus to open a Windows Terminal tab. The Windows installer
adds a Start Menu shortcut and a background task at Windows login.

This is an initial WSL2 port. It does not include a native Windows tray app or
discover sessions running in native Windows Claude Code, Codex, Orca or cmux.
The macOS menu bar app stays macOS-only. Codex sessions inside the selected
distribution are discovered read-only; open those sessions in their original
terminal. Focus is supported for agents started with `agentctl new`. Physical serial boards need
USB passthrough into WSL (for example usbipd-win); no Windows COM-port driver
is included.

## Install

1. Install WSL2 (`wsl --install` in an elevated PowerShell), restart if asked,
   and open the distribution once to create your Linux user. Check
   `wsl --list --verbose`: the distribution must show version 2. Install
   Windows Terminal from the Microsoft Store if it is missing.
2. Inside WSL, install `tmux`, `sqlite3`, and Go **1.24.1 or newer**. On Ubuntu,
   `sudo apt-get update && sudo apt-get install -y tmux sqlite3` installs the
   runtime dependencies; follow <https://go.dev/doc/install> for Go. Install
   and sign into Claude Code or Codex **inside WSL** as well.
3. From this checkout on Windows, run:

   ```powershell
   powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 -Distro Ubuntu
   ```

   Replace `Ubuntu` with the name in `wsl --list --verbose`, or omit `-Distro`
   to use your default distribution. `-NoStartup` starts a daemon for this
   session without registering a login task. `-NoOpen` skips opening the
   dashboard. The execution policy override applies only to that process.

The installer builds from this checkout, installs `~/.local/bin/agentctl` in
WSL, waits for its API to respond, and adds **agents deck (distribution)** to
the Windows Start Menu. Each shortcut click obtains a fresh one-time login
link. Nothing is added to the machine-wide Windows PATH.

In WSL:

```sh
export PATH="$HOME/.local/bin:$PATH"
cd ~/dev/my-project
agentctl new
# ctrl-q detaches; the agent keeps running
agentctl web
agentctl reopen
```

Add the PATH line to your WSL shell profile if needed. Windows folders are
available under `/mnt/`; projects under the WSL home directory generally have
better filesystem performance. Use Linux paths in `agentctl` commands.

## Startup and updates

Task Scheduler runs a hidden PowerShell process that keeps `wsl.exe` and the
daemon alive. It runs as your current Windows user, when that user logs in,
with no elevation, no task time limit, and retries on failure. Systemd is not
required. This keeps the selected WSL distribution running while the task is
active. Use one distribution for the deck: the dashboard's default port is
shared on the Windows host.

Rerun `install.ps1` from an updated checkout to rebuild and restart. The
installer stops the old daemon through its private socket; agents keep
running in tmux. The daemon refuses a second instance using the same private
data directory. If an older build lacks `agentctl stop`, stop that process
before updating.

`agentctl stop` stops the daemon without stopping agents. Run
`uninstall.ps1 -Distro Ubuntu` to stop it and remove the Windows startup task
and shortcut. The WSL binary, agents, and private data are kept. A daemon
started with `-NoStartup` can also be stopped with `agentctl stop`.

For a Linux systemd user service instead of Windows Task Scheduler, run
`agentctl install` in WSL. Use only one startup mechanism. A user service needs
a working systemd user session and Windows interop in its environment; the
PowerShell installer is the recommended Windows setup.

## Desktop integration and troubleshooting

- Focus opens a fresh Windows Terminal tab in the same distribution. Repeated
  Focus clicks can create several views of one agent. `agentctl set term tmux`
  switches the most recently used attached tmux client instead; it does not
  raise or select a Windows Terminal window. `agentctl reopen` opens only
  agents without an attached terminal.
- Clipboard selections go through Windows PowerShell's Unicode clipboard
  API. `agentctl set mouse native` leaves selection and copying to Windows
  Terminal instead.
- WSL interop and its Windows PATH must be enabled (the defaults). Check
  `command -v wt.exe` and `command -v powershell.exe` in WSL. If disabled, use
  `[interop] enabled=true` and `appendWindowsPath=true` in `/etc/wsl.conf`, then
  restart WSL and the deck. `wsl --shutdown` also stops running agents; use
  `agentctl restore` after restarting.
- The web server still binds **only to 127.0.0.1**, with the original login
  cookie, origin checks and CSP. WSL2's localhost forwarding lets the Windows
  browser reach it. If forwarding is disabled, enable it in `.wslconfig`;
  do not expose the dashboard on `0.0.0.0`.
- If opening the browser fails, `agentctl web` prints the login link so you
  can open it yourself. The link expires and is usable once.
- Logs are in `~/.local/share/agents-terminal/agentd.log`. Private state and
  keys remain in the WSL filesystem with Unix permissions. Do not relocate
  `AGENTSTERM_HOME` to a Windows drive without equivalent filesystem protection.
- Hand-started WSL Claude sessions can be watched with `agentctl live` and
  `agentctl add`; answer them in their original terminal. Background Claude
  sessions can be opened in a fresh Windows Terminal tab. Native Windows
  sessions are outside this port's discovery scope.

## Validation

CI runs Go vet, race tests and a daemon/agent lifecycle smoke test on Linux;
it also parses both PowerShell scripts on Windows. Host browser, clipboard,
Task Scheduler and Windows Terminal behavior need a Windows 11 / WSL2 machine
for manual validation. Check install, dashboard login, `new -c shell`, Focus,
clipboard copying, reinstall, Windows login startup and uninstall there.
