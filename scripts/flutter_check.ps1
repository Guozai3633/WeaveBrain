[CmdletBinding()]
param(
    [ValidateSet("all", "analyze", "test")]
    [string]$Task = "all",
    [switch]$PubGet
)

$ErrorActionPreference = "Stop"

$workspaceRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$flutterProjectRelative = "weave_flutter"
$selectedDrive = $null
$workRoot = $workspaceRoot
$mapped = $false
$pushed = $false

if ($workspaceRoot -match "[^\x00-\x7F]") {
    foreach ($candidate in @("W:", "V:", "U:")) {
        if (-not (Test-Path ($candidate + "\"))) {
            & subst.exe $candidate $workspaceRoot
            if ($LASTEXITCODE -eq 0) {
                $selectedDrive = $candidate
                $workRoot = $candidate + "\"
                $mapped = $true
                break
            }
        }
    }

    if (-not $mapped) {
        throw "No free ASCII drive letter is available for Flutter analysis."
    }
}

$analyzeExit = 0
$testExit = 0

try {
    Push-Location (Join-Path $workRoot $flutterProjectRelative)
    $pushed = $true

    if ($PubGet) {
        & flutter pub get
        if ($LASTEXITCODE -ne 0) {
            throw "flutter pub get failed with exit code $LASTEXITCODE"
        }
    }

    if ($Task -in @("all", "analyze")) {
        & flutter analyze --no-pub
        $analyzeExit = $LASTEXITCODE
    }

    if ($Task -in @("all", "test")) {
        & flutter test --no-pub
        $testExit = $LASTEXITCODE
    }
}
finally {
    if ($pushed) {
        Pop-Location
    }

    if ($mapped -and $selectedDrive) {
        & subst.exe $selectedDrive /D | Out-Null
    }
}

Write-Output "FLUTTER_ANALYZE_EXIT=$analyzeExit"
Write-Output "FLUTTER_TEST_EXIT=$testExit"

if ($analyzeExit -ne 0 -or $testExit -ne 0) {
    exit 1
}
