# Remove Windows startup and the shortcut. WSL agents and private data are kept.
[CmdletBinding()]
param([string]$Distro)
$ErrorActionPreference = 'Stop'
if (-not $Distro) {
    $Distro = ((& wsl.exe --exec printenv WSL_DISTRO_NAME) -join '').Trim()
    if ($LASTEXITCODE -ne 0 -or -not $Distro) { throw 'Pass -Distro <name>.' }
}
if ($Distro -notmatch '^[\p{L}\p{N}][\p{L}\p{N}._ -]*$') { throw 'Unsupported distribution name.' }
try {
    $ErrorActionPreference = 'Continue'
    & wsl.exe --distribution $Distro --exec /bin/sh -c 'exec ~/.local/bin/agentctl stop' *> $null
} finally {
    $ErrorActionPreference = 'Stop'
}
$taskName = 'agents-deck-' + $Distro
if (Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue) {
    Stop-ScheduledTask -TaskName $taskName
    Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
}
$shortcut = Join-Path ([Environment]::GetFolderPath('Programs')) ('agents deck (' + $Distro + ').lnk')
if (Test-Path -LiteralPath $shortcut) { Remove-Item -LiteralPath $shortcut }
Write-Host 'Removed Windows startup and the Start Menu shortcut. Agents, the WSL binary, and private data are kept.'
