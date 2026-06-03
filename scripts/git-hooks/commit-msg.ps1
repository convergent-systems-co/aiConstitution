param(
    [Parameter(Mandatory = $true)]
    [string] $MessageFile
)

$ErrorActionPreference = "Stop"
$message = Get-Content -LiteralPath $MessageFile -Raw
$aiAuthored = $message -match '(^|\s)(AI-authored|Generated-by:.*(AI|Codex|Copilot|Claude)|Co-authored-by:.*(OpenAI|Codex|Copilot|Claude))'
$hasTrailer = $message -match '(?m)^Co-authored-by: .+ <.+>$'

if ($aiAuthored -and -not $hasTrailer) {
    Write-Error "commit-msg: AI-authored commits require a Co-authored-by trailer"
    exit 1
}
