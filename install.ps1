# Install guppy, and leave guppy managing itself.
#
#   irm https://github.com/jaredhaight/guppy/releases/latest/download/install.ps1 | iex
#
# This script only bootstraps: it downloads a throwaway guppy to a temp
# directory, checks it against the release checksums, and then hands off to
# `guppy install jaredhaight/guppy`. Guppy installs itself through its own
# pipeline, so afterwards it is an ordinary managed app and `guppy update
# guppy` keeps it current.
$ErrorActionPreference = 'Stop'

$repo = 'jaredhaight/guppy'
$base = "https://github.com/$repo/releases/latest/download"

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default { throw "unsupported architecture: $($env:PROCESSOR_ARCHITECTURE) (guppy publishes amd64 and arm64)" }
}
$asset = "guppy-windows-$arch.exe"

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp | Out-Null

try {
    Write-Host "Downloading $asset..."
    # Invoke-WebRequest spends most of its time drawing the progress bar.
    $ProgressPreference = 'SilentlyContinue'
    Invoke-WebRequest "$base/$asset" -OutFile "$tmp\guppy.exe"
    Invoke-WebRequest "$base/checksums.txt" -OutFile "$tmp\checksums.txt"

    # checksums.txt proves only that the file matches what the release
    # published. The signed provenance attestation is the stronger check:
    #   gh attestation verify guppy.exe --repo jaredhaight/guppy
    $expected = $null
    foreach ($line in Get-Content "$tmp\checksums.txt") {
        $fields = $line -split '\s+'
        if ($fields.Count -ge 2 -and $fields[1] -eq $asset) { $expected = $fields[0] }
    }
    if (-not $expected) { throw "checksums.txt has no entry for $asset" }

    # -ne on strings is case-insensitive, which is what lets Get-FileHash's
    # uppercase hex meet sha256sum's lowercase.
    $actual = (Get-FileHash "$tmp\guppy.exe" -Algorithm SHA256).Hash
    if ($actual -ne $expected) {
        throw "checksum mismatch for ${asset}: expected $expected, got $actual"
    }
    Write-Host "* checksum verified"

    # Re-running the script should update rather than fail: `guppy install
    # owner/repo` refuses to overwrite an app that already has a config.
    #
    # Read the names out of the table only, starting after the header row. With
    # no apps configured guppy prints prose instead of a table, and that prose
    # suggests `guppy add ...` — close enough to a table row to match if you
    # just skip the first line.
    $rows = @(& "$tmp\guppy.exe" list 2>$null)
    $header = [array]::FindIndex([string[]]$rows, [Predicate[string]] { $args[0] -match '^APP\s' })
    $managed = if ($header -ge 0) {
        $rows[($header + 1)..($rows.Count - 1)] | ForEach-Object { ($_ -split '\s+')[0] }
    }

    if ($managed -contains 'guppy') {
        & "$tmp\guppy.exe" update guppy
    }
    else {
        # --bin is needed here and not on Unix: without it the installed file
        # is named "guppy" with no extension, which Windows won't run.
        & "$tmp\guppy.exe" install $repo --asset $asset --bin guppy.exe
    }
    if ($LASTEXITCODE -ne 0) { throw "guppy exited with $LASTEXITCODE" }

    $bin = & "$tmp\guppy.exe" bin
    if (($env:PATH -split ';') -notcontains $bin) {
        Write-Host ""
        Write-Host "Add guppy to your PATH:"
        Write-Host "  [Environment]::SetEnvironmentVariable('Path', '$bin;' + [Environment]::GetEnvironmentVariable('Path', 'User'), 'User')"
    }
    Write-Host ""
    Write-Host "Then run 'guppy update' to keep everything current, guppy included."
}
finally {
    Remove-Item -Recurse -Force $tmp
}
