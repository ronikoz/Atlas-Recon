#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Syncing Python plugins into internal/plugins..."

if [ ! -d "$PROJECT_ROOT/internal/plugins" ]; then
    mkdir -p "$PROJECT_ROOT/internal/plugins"
fi

if [ -d "$PROJECT_ROOT/internal/plugins/python" ]; then
    rm -rf "$PROJECT_ROOT/internal/plugins/python"
fi

cp -r "$PROJECT_ROOT/plugins/python" "$PROJECT_ROOT/internal/plugins/python"
rm -f "$PROJECT_ROOT/internal/plugins/python/README"*.md
