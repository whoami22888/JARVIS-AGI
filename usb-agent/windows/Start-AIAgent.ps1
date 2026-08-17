[CmdletBinding()]
param(
  [switch]$NoBrowser,
  [switch]$ForceRestart,
  [switch]$OfflineOnly,
  [switch]$RequireIntegrity
)

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $PSScriptRoot
$DataDir = Join-Path $Root 'data'
$RunDir = Join-Path $DataDir 'run'
$LogDir = Join-Path $DataDir 'logs'
$Python = Join-Path $Root 'runtime\python\python.exe'
$GatewayLog = Join-Path $LogDir 'gateway.log'
$GatewayPid = Join-Path $RunDir 'gateway.pid'
$Port = if ($env:USB_AGENT_PORT) { $env:USB_AGENT_PORT } else { '8787' }
$Url = "http://127.0.0.1:$Port"
$Manifest = Join-Path $Root 'release-manifest.json'

New-Item -ItemType Directory -Force -Path $RunDir, $LogDir | Out-Null
if (-not (Test-Path $Python)) {
  throw "Portable Python runtime was not found at $Python. Run the USB preparation process before launching."
}
if ($RequireIntegrity -and -not (Test-Path $Manifest)) {
  throw "Release manifest was not found at $Manifest. Refusing integrity-required launch."
}

function Test-AgentHealth {
  try { return (Invoke-WebRequest -UseBasicParsing -TimeoutSec 2 "$Url/health").StatusCode -eq 200 }
  catch { return $false }
}

if ($ForceRestart -and (Test-Path $GatewayPid)) {
  $pidValue = Get-Content $GatewayPid -ErrorAction SilentlyContinue
  if ($pidValue) { Stop-Process -Id $pidValue -Force -ErrorAction SilentlyContinue }
  Remove-Item $GatewayPid -Force -ErrorAction SilentlyContinue
}

if (-not (Test-AgentHealth)) {
  $env:USB_AGENT_HOME = $Root
  $env:USB_AGENT_DATA = $DataDir
  $env:USB_AGENT_CONFIG = Join-Path $Root 'config'
  $env:PYTHONPATH = Join-Path $Root 'backend'
  $env:USB_AGENT_OFFLINE_ONLY = if ($OfflineOnly) { '1' } else { '0' }
  $arguments = '-m uvicorn app:app --app-dir "{0}" --host 127.0.0.1 --port {1}' -f (Join-Path $Root 'backend'), $Port
  $process = Start-Process -FilePath $Python -ArgumentList $arguments -WorkingDirectory $Root -RedirectStandardOutput $GatewayLog -RedirectStandardError $GatewayLog -PassThru -WindowStyle Hidden
  Set-Content -Path $GatewayPid -Value $process.Id

  $ready = $false
  1..30 | ForEach-Object {
    Start-Sleep -Seconds 1
    if (Test-AgentHealth) { $script:ready = $true; return }
  }
  if (-not $ready) { throw "The local gateway did not start. Check $GatewayLog" }
}

if (-not $NoBrowser) { Start-Process $Url }
Write-Host "USB Agent is ready at $Url (offline-only: $OfflineOnly)"
