#!/bin/sh
# Real Unix-socket API + tmux lifecycle, without AI tools or credentials.
set -eu
root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
export AGENTSTERM_HOME="$tmp/data"
pid=""
id=""
cleanup() {
  if [ -n "$id" ]; then "$tmp/agentctl" rm "$id" >/dev/null 2>&1 || true; fi
  "$tmp/agentctl" stop >/dev/null 2>&1 || true
  if [ -n "$pid" ]; then wait "$pid" 2>/dev/null || true; fi
  rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM
mkdir -p "$AGENTSTERM_HOME"
printf '%s\n' '{"deviceAddr":""}' > "$AGENTSTERM_HOME/config.json"
(cd "$root/backend" && go build -o "$tmp/agentctl" ./cmd/agentctl)
"$tmp/agentctl" serve >"$tmp/daemon.log" 2>&1 &
pid=$!
n=0
until "$tmp/agentctl" ls >/dev/null 2>&1; do
  n=$((n + 1))
  if [ "$n" -gt 50 ]; then cat "$tmp/daemon.log" >&2; exit 1; fi
  sleep 0.1
done
# A duplicate must fail before it can unlink the running daemon's socket.
if "$tmp/agentctl" serve >"$tmp/duplicate.log" 2>&1; then
  echo "duplicate daemon was accepted" >&2
  exit 1
fi
id="$("$tmp/agentctl" new "$tmp" -c shell -d)"
"$tmp/agentctl" ls | grep -F "$id" >/dev/null
"$tmp/agentctl" rm "$id"
id=""
"$tmp/agentctl" stop
wait "$pid"
pid=""
echo "Linux daemon, duplicate protection, agent creation/removal and shutdown passed"
