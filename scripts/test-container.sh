#!/bin/bash

# Quick container test script
# Tests that the container can be built and basic functionality works

set -e

echo "🐳 Testing AgentBox Container"
echo "=============================="
echo ""

# Build image
echo "1. Building Docker image..."
docker build -t agentbox:test -f Dockerfile . > /dev/null 2>&1
echo "   ✅ Image built successfully"
echo ""

# Test file structure
echo "2. Testing container file structure..."
docker run --rm --entrypoint sh agentbox:test -c "
    test -f /app/agentbox && echo '✅ Binary exists' || echo '❌ Binary missing'
    test -f /app/config/config.yaml && echo '✅ Config exists' || echo '❌ Config missing'
    whoami | grep -q agentbox && echo '✅ Running as non-root' || echo '❌ Not running as non-root'
"
echo ""

# Test help command
echo "3. Testing application help..."
if docker run --rm agentbox:test --help 2>&1 | grep -qE "(config|Usage)" > /dev/null 2>&1; then
    echo "   ✅ Help command works"
else
    echo "   ❌ Help command failed"
fi
echo ""

# Test image size
echo "4. Image information:"
docker images agentbox:test --format "   Size: {{.Size}}"
echo ""

echo "✅ Container tests complete!"
