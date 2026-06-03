param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]] $HookArgs
)

$ErrorActionPreference = "Stop"
& ai guard --git-hook pre-push @HookArgs
exit $LASTEXITCODE
