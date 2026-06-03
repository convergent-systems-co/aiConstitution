$ErrorActionPreference = "Stop"
& ai guard --git-hook pre-commit
exit $LASTEXITCODE
