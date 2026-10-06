#!/bin/bash
set -euo pipefail

# FinFocus Vantage Plugin - Demo Script
# Runs the plugin against Wiremock mock server and displays sample records

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== FinFocus Vantage Plugin Demo ===${NC}"
echo "Script directory: $SCRIPT_DIR"
echo "Project directory: $PROJECT_DIR"

# Build the binary if it doesn't exist
if [ ! -f "$PROJECT_DIR/bin/finfocus-vantage" ]; then
    echo -e "${BLUE}Building binary...${NC}"
    cd "$PROJECT_DIR"
    make build
fi

# Check if Wiremock is running
if ! curl -s http://localhost:8080/__admin/mappings > /dev/null 2>&1; then
    echo -e "${BLUE}Wiremock not running. Starting...${NC}"
    cd "$PROJECT_DIR"
    make wiremock-up
    # Wait for Wiremock to be ready
    sleep 2
fi

# Run the CLI against mock server
echo -e "${GREEN}Running finfocus-vantage pull...${NC}"
cd "$PROJECT_DIR"

if [ -f "config.example.yaml" ]; then
    # Use environment variable for mock server URL
    VANTAGE_URL="http://localhost:8080" \
    ./bin/finfocus-vantage pull --config config.example.yaml || true
else
    echo "Warning: config.example.yaml not found"
    ./bin/finfocus-vantage pull --help
fi

echo -e "${GREEN}Demo complete!${NC}"
