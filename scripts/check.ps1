[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'common.ps1')
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$go = Get-RequiredTool 'go' 'Install the Go version declared in go.mod.'
$bun = Get-RequiredTool 'bun' 'Install Bun to run the Worker JavaScript tests.'
$null = Get-RequiredTool 'node' 'Install a Node.js version supported by Wrangler.'
$null = Get-WranglerPath $root

Invoke-RepositoryTask $root {
    Invoke-CheckedTool $go @('test', './...')
    Invoke-CheckedTool $go @('vet', './...')
    Build-WorkerWasm $root $go
    # The JavaScript suite includes real Go WASM fixtures and a Wrangler dry-run
    # that checks the actual delivered browser module, not only source syntax.
    Invoke-CheckedTool $bun @('test', './worker/src')
    Write-Host 'All checks passed (Go tests, go vet, WASM, JavaScript, Wrangler bundle).'
}
