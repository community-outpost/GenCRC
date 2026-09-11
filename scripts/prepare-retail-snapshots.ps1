[CmdletBinding()]
param(
    [ValidateSet('generals', 'generalsmd')][string]$Game,
    [string]$GeneralsRoot,
    [string]$ZeroHourRoot,
    [string]$Output,
    [switch]$Verify
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'common.ps1')
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$go = Get-RequiredTool 'go' 'Install the Go version declared in go.mod.'
if (-not $Output) { $Output = Join-Path $root 'worker/retail' }
# Resolve explicit relative paths against the caller's directory, not the repo.
$Output = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Output)
if ($GeneralsRoot) { $GeneralsRoot = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($GeneralsRoot) }
if ($ZeroHourRoot) { $ZeroHourRoot = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($ZeroHourRoot) }
$arguments = @('run', './cmd/gencrc-snapshots', '--output', $Output)
if ($Game) { $arguments += @('--game', $Game) }
if ($GeneralsRoot) { $arguments += @('--generals-root', $GeneralsRoot) }
if ($ZeroHourRoot) { $arguments += @('--zero-hour-root', $ZeroHourRoot) }
if ($Verify) { $arguments += '--verify' }
Invoke-RepositoryTask $root { Invoke-CheckedTool $go $arguments }
