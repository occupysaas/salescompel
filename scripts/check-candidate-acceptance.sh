#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
exec compat/clients/node/node_modules/.bin/tsx compat/clients/node/src/candidate-acceptance-command.ts full "$@"
