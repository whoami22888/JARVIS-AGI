[CmdletBinding()]
param(
  [ValidateSet('system_diagnostics')]
  [string]$Action = 'system_diagnostics'
)

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $PSScriptRoot
$AuditDir = Join-Path $Root 'data\approvals'
New-Item -ItemType Directory -Force -Path $AuditDir | Out-Null

if (-not ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  $args = "-ExecutionPolicy Bypass -File `"$PSCommandPath`" -Action $Action"
  Start-Process -FilePath 'powershell.exe' -Verb RunAs -ArgumentList $args
  exit 0
}

$timestamp = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ')
$audit = Join-Path $AuditDir "elevated-$Action-$timestamp.json"

switch ($Action) {
  'system_diagnostics' {
    $result = [ordered]@{
      action = $Action
      executed_at_utc = (Get-Date).ToUniversalTime().ToString('o')
      computer = $env:COMPUTERNAME
      is_elevated = $true
      os = Get-CimInstance Win32_OperatingSystem | Select-Object Caption, Version, BuildNumber, LastBootUpTime
      disks = Get-CimInstance Win32_LogicalDisk -Filter 'DriveType=3' | Select-Object DeviceID, Size, FreeSpace
      note = 'Diagnostics only. No settings or security controls were changed.'
    }
    $result | ConvertTo-Json -Depth 4 | Set-Content -Path $audit -Encoding UTF8
    Write-Host "Completed elevated diagnostics. Audit record: $audit"
  }
}
