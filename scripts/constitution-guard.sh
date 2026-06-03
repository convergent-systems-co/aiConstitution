#!/bin/sh
set -eu
exec go run ./src/cmd/ai guard "$@"
