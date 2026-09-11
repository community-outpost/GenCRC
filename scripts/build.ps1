[CmdletBinding()]
param([ValidateSet('all', 'cli', 'worker')][string]$Target = 'all')

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'common.ps1')
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$go = Get-RequiredTool 'go' 'Install the Go version declared in go.mod.'
if ($Target -ne 'cli') {
    $node = Get-RequiredTool 'node' 'Install a Node.js version supported by Wrangler.'
    $wrangler = Get-WranglerPath $root
}

Invoke-RepositoryTask $root {
    if ($Target -ne 'worker') {
        $output = Join-Path $root 'dist/cli'
        New-Item -ItemType Directory -Force -Path $output | Out-Null
        $hostOS = & $go env GOHOSTOS
        if ($LASTEXITCODE -ne 0) { throw 'Could not determine the native Go operating system.' }
        $name = 'gencrc'
        if ($hostOS -eq 'windows') { $name += '.exe' }
        Invoke-CheckedTool $go @('build', '-trimpath', '-ldflags=-s -w', '-o', (Join-Path $output $name), './cmd/gencrc')
        Write-Host "Built CLI: $(Join-Path $output $name)"
    }
    if ($Target -ne 'cli') {
        Build-WorkerWasm $root $go
        Build-WorkerBundle $root $node $wrangler
    }
}
