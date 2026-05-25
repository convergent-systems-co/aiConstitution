@echo off
:: notify-me.cmd — CMD batch shim for the notify-me PowerShell script.
::
:: Delegates all argument handling to notify-me.ps1 in the same directory.
:: Uses -ExecutionPolicy Bypass so the script runs without requiring the
:: user to change their system execution policy.
::
:: Usage:
::   notify-me.cmd --title <str> --message <str> [--level info|warn|urgent]
::
:: Per issue #245.

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0notify-me.ps1" %*
exit /b %ERRORLEVEL%
