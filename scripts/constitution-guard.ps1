param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]] $GuardArgs
)

$ErrorActionPreference = "Stop"
& go run ./src/cmd/ai guard @GuardArgs
exit $LASTEXITCODE
