#!/bin/bash
set -e

echo "=== Installing GPTCode Neovim Plugin ==="
echo ""

if ! command -v nvim &> /dev/null; then
    echo "Error: Neovim is not installed"
    exit 1
fi

if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    exit 1
fi

NVIM_CONFIG="${XDG_CONFIG_HOME:-$HOME/.config}/nvim"
if [ ! -d "$NVIM_CONFIG" ]; then
    echo "Error: Neovim config directory not found at $NVIM_CONFIG"
    exit 1
fi

# Create plugin directory structure
PLUGIN_DIR="$NVIM_CONFIG/pack/plugins/start/gptcode"
echo "1. Creating plugin directory: $PLUGIN_DIR"
mkdir -p "$PLUGIN_DIR/lua/gptcode"
mkdir -p "$PLUGIN_DIR/plugin"

# Copy lua files
echo "2. Copying Lua files..."
cp -v ./neovim/lua/gptcode/init.lua "$PLUGIN_DIR/lua/gptcode/init.lua"
cp -v ./neovim/lua/gptcode/init.lua "$PLUGIN_DIR/lua/gptcode.lua"

# Copy plugin init script (auto-registers setup on Neovim startup)
echo "3. Copying Neovim plugin script..."
cp -v ./neovim/plugin/gptcode.vim "$PLUGIN_DIR/plugin/gptcode.vim"

# Ensure binary is in place
BIN_DEST="$HOME/.local/bin/gptcode"
mkdir -p "$HOME/.local/bin"
echo "4. Installing gptcode binary to $BIN_DEST..."
go build -o "$BIN_DEST" ./cmd/gptcode
echo "✓ Binary built and installed at $BIN_DEST"

echo ""
echo "✓ Plugin installed successfully!"
echo ""
echo "Next steps:"
echo "1. Restart Neovim"
echo "2. Test commands: :GPTCodeChat or :GPTCodeAuto"
echo ""
echo "The plugin binary is active at: $BIN_DEST"
