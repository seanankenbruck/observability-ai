#!/bin/bash

# Claude Client Test Script

set -e

echo "=== Claude Client Test Setup ==="

# Check if API key is set
if [ -z "$CLAUDE_API_KEY" ]; then
    echo "❌ CLAUDE_API_KEY environment variable is not set"
    echo ""
    echo "Please set your Claude API key:"
    echo "export CLAUDE_API_KEY='your-api-key-here'"
    echo ""
    echo "You can get your API key from: https://console.anthropic.com/"
    exit 1
fi

echo "✓ CLAUDE_API_KEY is set"

# Check if Go modules are ready
echo "Checking Go modules..."
go mod tidy
echo "✓ Go modules ready"

# Build the test program
echo "Building test program..."
go build -o bin/test-llm cmd/test-llm/main.go
echo "✓ Test program built"

# Run the test
echo ""
echo "Running Claude client tests..."
echo "================================"
./bin/test-llm

echo ""
echo "Test completed! Check the output above for any issues."