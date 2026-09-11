# Shared helpers for repository tasks. Keep this compatible with PowerShell 5.1.
function Get-RequiredTool {
    param([string]$Name, [string]$Hint)
    $tool = Get-Command $Name -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $tool) { throw "Required tool '$Name' was not found on PATH. $Hint" }
    return $tool.Source
}

function Invoke-CheckedTool {
    param([string]$Command, [string[]]$Arguments)
    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) { throw "Command failed with exit code ${LASTEXITCODE}: $Command $($Arguments -join ' ')" }
}

function Invoke-RepositoryTask {
    param([string]$Root, [scriptblock]$Task)
    $names = @('GOOS', 'GOARCH', 'GOCACHE', 'GOMODCACHE', 'WRANGLER_LOG_PATH', 'WRANGLER_SEND_METRICS')
    $previous = @{}
    foreach ($name in $names) { $previous[$name] = [Environment]::GetEnvironmentVariable($name, 'Process') }
    Push-Location -LiteralPath $Root
    try {
        # Native commands must not inherit a previous WASM cross-compile target.
        $env:GOOS = $null
        $env:GOARCH = $null
        if (-not $env:GOCACHE) { $env:GOCACHE = Join-Path $Root '.gocache' }
        if (-not $env:GOMODCACHE) { $env:GOMODCACHE = Join-Path $Root '.gomodcache' }
        if (-not $env:WRANGLER_LOG_PATH) { $env:WRANGLER_LOG_PATH = Join-Path $Root 'worker/generated/wrangler.log' }
        $env:WRANGLER_SEND_METRICS = 'false'
        & $Task
    } finally {
        foreach ($name in $names) { [Environment]::SetEnvironmentVariable($name, $previous[$name], 'Process') }
        Pop-Location
    }
}

function Build-WorkerWasm {
    param([string]$Root, [string]$Go)
    $output = Join-Path $Root 'worker/generated'
    New-Item -ItemType Directory -Force -Path $output | Out-Null
    $previousOS, $previousArch = $env:GOOS, $env:GOARCH
    try {
        $env:GOOS = 'js'
        $env:GOARCH = 'wasm'
        Invoke-CheckedTool $Go @('build', '-trimpath', '-ldflags=-s -w', '-o', (Join-Path $output 'gencrc.wasm'), './cmd/gencrc-wasm')
        $goRoot = & $Go env GOROOT
        if ($LASTEXITCODE -ne 0) { throw 'Could not determine GOROOT.' }
        Copy-Item -LiteralPath (Join-Path $goRoot 'lib/wasm/wasm_exec.js') -Destination (Join-Path $output 'wasm_exec.js') -Force
    } finally {
        $env:GOOS, $env:GOARCH = $previousOS, $previousArch
    }
}

function Get-WranglerPath {
    param([string]$Root)
    $wrangler = Join-Path $Root 'worker/node_modules/wrangler/bin/wrangler.js'
    if (-not (Test-Path -LiteralPath $wrangler -PathType Leaf)) { throw 'Worker dependencies are missing. Run npm ci --prefix worker first.' }
    return $wrangler
}

function Assert-BundleDirectory {
    param([string]$Path)
    if (-not (Get-Item -LiteralPath $Path -Force -ErrorAction SilentlyContinue)) { return }
    $pending = New-Object 'System.Collections.Generic.Stack[string]'
    $pending.Push($Path)
    while ($pending.Count -gt 0) {
        $item = Get-Item -LiteralPath $pending.Pop() -Force
        if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw "Refusing to replace a linked bundle path: $($item.FullName)" }
        if (-not $item.PSIsContainer) { throw "Bundle output must be a directory: $($item.FullName)" }
        foreach ($child in (Get-ChildItem -LiteralPath $item.FullName -Force)) {
            if ($child.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw "Refusing to replace a linked bundle entry: $($child.FullName)" }
            if ($child.PSIsContainer) { $pending.Push($child.FullName) }
        }
    }
}

function Remove-OwnedBundleDirectory {
    param([string]$Dist, [string]$Path)
    $absolute = [IO.Path]::GetFullPath($Path)
    $parent = Get-Item -LiteralPath $Dist -Force
    if (-not $parent.PSIsContainer -or ($parent.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw "Refusing to clean a redirected build output parent: $Dist" }
    if ([IO.Path]::GetDirectoryName($absolute) -ne [IO.Path]::GetFullPath($Dist) -or
        [IO.Path]::GetFileName($absolute) -notmatch '^\.worker-(build|previous)-[a-f0-9]{32}$') {
        throw "Refusing to remove a directory outside this build's staging paths: $absolute"
    }
    Assert-BundleDirectory $absolute
    Remove-Item -LiteralPath $absolute -Recurse -Force
}

function Build-WorkerBundle {
    param([string]$Root, [string]$Node, [string]$Wrangler)
    $dist = [IO.Path]::GetFullPath((Join-Path $Root 'dist'))
    # Check the parent itself without following a redirected output directory.
    $item = Get-Item -LiteralPath $dist -Force -ErrorAction SilentlyContinue
    if ($item) {
        if (-not $item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw "Build output parent must be an ordinary directory: $dist" }
    }
    New-Item -ItemType Directory -Force -Path $dist | Out-Null
    $output = Join-Path $dist 'worker'
    Assert-BundleDirectory $output
    $id = [Guid]::NewGuid().ToString('N')
    $staging = Join-Path $dist ".worker-build-$id"
    $backup = Join-Path $dist ".worker-previous-$id"
    New-Item -ItemType Directory -Path $staging | Out-Null
    $installed, $hadPrevious = $false, $false
    try {
        Invoke-CheckedTool $Node @($Wrangler, 'deploy', '--dry-run', '--config', (Join-Path $Root 'worker/wrangler.jsonc'), '--outdir', $staging)
        if (-not (Test-Path -LiteralPath (Join-Path $staging 'index.js') -PathType Leaf)) { throw 'Wrangler completed without producing index.js; previous bundle was preserved.' }
        Assert-BundleDirectory $output
        if (Test-Path -LiteralPath $output) {
            Move-Item -LiteralPath $output -Destination $backup
            $hadPrevious = $true
        }
        try {
            Move-Item -LiteralPath $staging -Destination $output
            $installed = $true
        } catch {
            if ($hadPrevious) {
                try { Move-Item -LiteralPath $backup -Destination $output }
                catch { throw "Bundle replacement and rollback failed. Previous output is preserved at $backup. $($_.Exception.Message)" }
            }
            throw
        }
    } finally {
        if (Test-Path -LiteralPath $staging) { Remove-OwnedBundleDirectory $dist $staging }
        if ($installed -and $hadPrevious) { Remove-OwnedBundleDirectory $dist $backup }
    }
    Write-Host "Built Worker bundle: $output"
}
