# Compatibility entrypoint for dev/test consumers that need only the WASM files.
# Use build.ps1 -Target worker for the complete Wrangler deployment bundle.
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'common.ps1')
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$go = Get-RequiredTool 'go' 'Install the Go version declared in go.mod.'
Invoke-RepositoryTask $root { Build-WorkerWasm $root $go }

