#!/bin/bash
# Build script for CLI-TOOLS with embedded plugins
# This script ensures Python plugins are copied to the internal directory
# for embedding into the binary

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"

echo "🔨 Building CLI-Tools with embedded plugins..."

# Step 1: Ensure internal/plugins directory exists
if [ ! -d "$PROJECT_ROOT/internal/plugins" ]; then
    echo "📁 Creating internal/plugins directory..."
    mkdir -p "$PROJECT_ROOT/internal/plugins"
fi

# Step 2: Copy Python plugins (refresh each build to ensure latest)
echo "📦 Copying Python plugins to internal/plugins..."
if [ -d "$PROJECT_ROOT/internal/plugins/python" ]; then
    rm -rf "$PROJECT_ROOT/internal/plugins/python"
fi
cp -r "$PROJECT_ROOT/plugins/python" "$PROJECT_ROOT/internal/plugins/python"

# Step 3: Remove README files to reduce binary size (optional)
# Comment this out to keep README files
rm -f "$PROJECT_ROOT/internal/plugins/python/README*.md"

# Step 4: Build the binary
echo "🏗️  Building binary..."
cd "$PROJECT_ROOT"
go build -o ct ./cmd/ct

# Step 5: Report success
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
