# notify-me.ps1 — Windows OS notification wrapper for the `ai` system.
#
# Usage:
#   notify-me.ps1 --title <str> --message <str> [--level info|warn|urgent]
#
# Dispatch priority:
#   1. BurntToast module (New-BurntToastNotification) — if installed.
#   2. System.Windows.Forms.MessageBox — always available on Windows.
#
# Exit behavior:
#   - Success: exits 0, nothing written to stdout.
#   - Failure: exits non-zero, error written to stderr.
#
# Per issue #245.

param(
    [Parameter(Mandatory = $true)]
    [string]$Title,

    [Parameter(Mandatory = $true)]
    [string]$Message,

    [ValidateSet('info', 'warn', 'urgent')]
    [string]$Level = 'info'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Invoke-Notification {
    param(
        [string]$NotifTitle,
        [string]$NotifMessage
    )

    # Priority 1: BurntToast (toast notification — non-blocking, native Windows 10/11 toast).
    if (Get-Command -Name 'New-BurntToastNotification' -ErrorAction SilentlyContinue) {
        New-BurntToastNotification -Text $NotifTitle, $NotifMessage
        return
    }

    # Priority 2: System.Windows.Forms.MessageBox (modal, always available).
    Add-Type -AssemblyName 'System.Windows.Forms'
    [System.Windows.Forms.MessageBox]::Show($NotifMessage, $NotifTitle) | Out-Null
}

try {
    Invoke-Notification -NotifTitle $Title -NotifMessage $Message
    exit 0
}
catch {
    Write-Error "notify-me: notification failed: $_"
    exit 1
}
