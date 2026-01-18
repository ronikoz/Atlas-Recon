#!/bin/bash
# Install OSINT dependencies for CLI Tools (Mac/Homebrew/Pip)

set -e

echo "Received request to install OSINT dependencies..."

if ! command -v brew &> /dev/null; then
    echo "Error: Homebrew is required (https://brew.sh/)"
    exit 1
fi

echo "[*] Updating Homebrew..."
# brew update

echo "[*] Installing core tools via Homebrew..."
brew install masscan trufflehog gitleaks exiftool

echo "[*] Installing ProjectDiscovery tools (subfinder, naabu, katana)..."
brew install projectdiscovery/tap/subfinder
brew install projectdiscovery/tap/naabu
brew install projectdiscovery/tap/katana
# brew install projectdiscovery/tap/httpx

echo "[*] Installing OWASP Amass..."
brew install amass

echo "[*] Installing Go tools (waybackurls)..."
if command -v go &> /dev/null; then
    go install github.com/tomnomnom/waybackurls@latest
    # Ensure ~/go/bin is in PATH
    export PATH=$PATH:$(go env GOPATH)/bin
else
    echo "Warning: Go not found, skipping waybackurls"
fi

echo "[*] Installing Python tools (sherlock, maigret, holehe, metagoofil)..."
# Ensure we use the same python as the CLI tool if possible, or system pip3
pip3 install sherlock-project maigret holehe metagoofil snscrape requests

echo ""
echo "----------------------------------------"
echo "Installation complete!"
echo "Please ensure your PATH includes Go binaries (usually ~/go/bin)."
echo "You can now run 'ct dashboard' and use 'osint suite'."
echo "----------------------------------------"
