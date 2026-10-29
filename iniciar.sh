#!/usr/bin/env sh
cd "$(dirname "$0")"
command -v node >/dev/null 2>&1 || { echo "Instale o Node.js 18 ou mais novo: https://nodejs.org"; exit 1; }
CENTRAL_ABRIR=1 exec node servidor/index.mjs
