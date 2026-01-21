#!/bin/bash
# Build script for Atlas-Recon with embedded plugins
# This script ensures Python plugins are copied to the internal directory
# for embedding into the binary

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"

echo "🔨 Building Atlas-Recon with embedded plugins..."

# Step 1: Sync Python plugins from source of truth
"$PROJECT_ROOT/scripts/sync_plugins.sh"

# Step 2: Build the binary
echo "🏗️  Building binary..."
cd "$PROJECT_ROOT"
go build -o ct ./cmd/ct

# Step 3: Report success
BINARY_SIZE=$(du -h ct | cut -f1)
echo "✅ Build successful!"
echo "📊 Binary size: $BINARY_SIZE"
echo "📍 Location: $PROJECT_ROOT/ct"
echo ""
echo "ℹ️  Plugins are now embedded in the binary and will be extracted to:"
echo "   ~/.cache/ct_plugins/"
echo ""
echo "🚀 You can now run the binary from any directory:"
echo "   $PROJECT_ROOT/ct scan example.com"
echo "   /path/to/ct dns example.com"
