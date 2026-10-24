# Install from this checkout into a WSL2 distribution. Run in Windows PowerShell
# or PowerShell 7: .\install.ps1 [-Distro Ubuntu] [-NoStartup] [-NoOpen]
[CmdletBinding()]
param(
    [string]$Distro,
    [switch]$NoStartup,
    [switch]$NoOpen
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if (-not (Get-Command wsl.exe -ErrorAction SilentlyContinue)) {
    throw 'Install WSL2 first: wsl --install. Restart Windows and finish creating your Linux user, then run this installer again.'
}
if (-not $Distro) {
    $Distro = ((& wsl.exe --exec printenv WSL_DISTRO_NAME) -join '').Trim()
    if ($LASTEXITCODE -ne 0 -or -not $Distro) { throw 'Start WSL and finish setting up your Linux user, or pass -Distro <name>.' }
}
if ($Distro -notmatch '^[\p{L}\p{N}][\p{L}\p{N}._ -]*$') { throw 'Use a distribution name containing letters, digits, spaces, dots, underscores or hyphens.' }
$wslArgs = @('--distribution', $Distro, '--exec')
$kernel = ((& wsl.exe @wslArgs uname -r) -join '').Trim()
if ($LASTEXITCODE -ne 0 -or $kernel -notmatch '(?i)microsoft.*(standard|wsl2)') {
    throw "Select a WSL2 distribution. Check wsl --list --verbose; convert with wsl --set-version `"$Distro`" 2."
}
if (-not (Get-Command wt.exe -ErrorAction SilentlyContinue)) {
    throw 'Install Windows Terminal from the Microsoft Store, then reopen PowerShell.'
}

# Build inside WSL. No Windows Go toolchain or Flutter installation is needed.
$source = ((& wsl.exe @wslArgs wslpath -a -u $PSScriptRoot) -join '').Trim()
if ($LASTEXITCODE -ne 0 -or -not $source) { throw 'Cannot map the checkout into WSL. Enable Windows drive mounting in /etc/wsl.conf.' }
& wsl.exe @wslArgs /bin/bash -lc 'command -v tmux >/dev/null && command -v sqlite3 >/dev/null && command -v go >/dev/null'
if ($LASTEXITCODE -ne 0) {
    throw 'Inside WSL, install tmux and sqlite3 (sudo apt-get update && sudo apt-get install -y tmux sqlite3), and Go 1.24.1 or newer from https://go.dev/doc/install. Then rerun this installer.'
}
& wsl.exe --distribution $Distro --cd $source --exec /bin/bash -lc 'sh ./install.sh --source --no-daemon'
if ($LASTEXITCODE -ne 0) { throw 'The WSL source build failed. Go 1.24.1 or newer is required.' }
$linuxHome = ((& wsl.exe @wslArgs printenv HOME) -join '').Trim()
if ($LASTEXITCODE -ne 0 -or -not $linuxHome.StartsWith('/')) { throw 'Cannot determine your WSL home directory.' }
$binary = $linuxHome + '/.local/bin/agentctl'
& wsl.exe @wslArgs $binary set term windows
if ($LASTEXITCODE -ne 0) { throw 'Could not configure Windows Terminal integration.' }

# A foreground wsl.exe task keeps WSL alive and gives Task Scheduler a process
# to supervise. It works with or without systemd inside the distribution.
$launcher = @'
set -eu
umask 077
mkdir -p "$HOME/.local/share/agents-terminal"
cat > "$HOME/.local/share/agents-terminal/windows-serve.sh" <<'SERVE'
#!/bin/sh
umask 077
exec "$HOME/.local/bin/agentctl" serve >>"$HOME/.local/share/agents-terminal/agentd.log" 2>&1
SERVE
chmod 700 "$HOME/.local/share/agents-terminal/windows-serve.sh"
'@
$launcher | & wsl.exe @wslArgs /bin/sh -c 'tr -d ''\r'' | /bin/sh -s'
if ($LASTEXITCODE -ne 0) { throw 'Could not write the daemon launcher in WSL.' }
$serveScript = $linuxHome + '/.local/share/agents-terminal/windows-serve.sh'
$taskName = 'agents-deck-' + $Distro
function Quote-PowerShell([string]$Value) { return "'" + $Value.Replace("'", "''") + "'" }
$daemonScript = '& wsl.exe --distribution ' + (Quote-PowerShell $Distro) + ' --exec /bin/sh ' + (Quote-PowerShell $serveScript) + '; exit $LASTEXITCODE'
$encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($daemonScript))
$taskArgs = '-NoProfile -NonInteractive -WindowStyle Hidden -EncodedCommand ' + $encoded
$windowsPowerShell = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'

# A missing/stopped daemon is expected during installation. Windows PowerShell
# 5.1 wraps redirected native stderr as error records, so silence these probes
# in a local scope without changing failure handling for installation itself.
function Invoke-AgentQuiet([string]$Command) {
    $ErrorActionPreference = 'Continue'
    & wsl.exe @wslArgs $binary $Command *> $null
    return ($LASTEXITCODE -eq 0)
}
# Stop the previous daemon through its private socket so updates flush state.
[void](Invoke-AgentQuiet 'stop')
if (Invoke-AgentQuiet 'ls') {
    throw 'An older daemon is still running. Stop its terminal or systemd service, then rerun this installer. No new startup task has been registered.'
}

if (-not $NoStartup) {
    $userId = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
    $action = New-ScheduledTaskAction -Execute $windowsPowerShell -Argument $taskArgs
    $trigger = New-ScheduledTaskTrigger -AtLogOn -User $userId
    $principal = New-ScheduledTaskPrincipal -UserId $userId -LogonType Interactive -RunLevel Limited
    $settings = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) -MultipleInstances IgnoreNew -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
    # Stop a previous task before updating so it uses the newly built binary.
    $previous = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
    if ($previous) { Stop-ScheduledTask -TaskName $taskName }
    Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Description 'Local agents deck daemon in WSL2' -Force | Out-Null
    Start-ScheduledTask -TaskName $taskName
} else {
    Start-Process -FilePath $windowsPowerShell -ArgumentList $taskArgs -WindowStyle Hidden | Out-Null
}

# Wait for the Unix-socket API, rather than claiming success when a task merely
# started. A failed build/startup leaves the log available for diagnosis.
$ready = $false
for ($attempt = 0; $attempt -lt 30; $attempt++) {
    if (Invoke-AgentQuiet 'ls') { $ready = $true; break }
    Start-Sleep -Milliseconds 500
}
if (-not $ready) { throw 'The daemon did not become ready. Read ~/.local/share/agents-terminal/agentd.log inside WSL; stop any older agentctl serve or systemd service before retrying.' }

# A Start Menu shortcut obtains a fresh one-time login link on each click.
$startMenu = Join-Path ([Environment]::GetFolderPath('Programs')) ('agents deck (' + $Distro + ').lnk')
$shortcut = (New-Object -ComObject WScript.Shell).CreateShortcut($startMenu)
$shortcut.TargetPath = (Get-Command wsl.exe).Source
$shortcut.Arguments = '--distribution "' + $Distro + '" --exec "' + $binary + '" web'
$shortcut.WindowStyle = 7
$shortcut.Description = 'Open the agents deck dashboard'
$shortcut.Save()
Write-Host "Installed agents deck in $Distro. Open it from the Start Menu."
Write-Host 'Start agents inside WSL: ~/.local/bin/agentctl new; open the dashboard: ~/.local/bin/agentctl web'
if (-not $NoOpen) {
    & wsl.exe @wslArgs $binary web
    if ($LASTEXITCODE -ne 0) { Write-Warning 'Could not open the browser. Run agentctl web in WSL and open its printed login link.' }
}
