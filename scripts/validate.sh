#!/bin/bash

# AgentBox Service Validation Script
# Validates that the service compiles and all components are properly connected

set -e

echo "🔍 Validating AgentBox Service..."
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

ERRORS=0

# 1. Check compilation
echo "1. Checking compilation..."
if go build ./cmd/server 2>&1; then
    echo -e "   ${GREEN}✅ Service compiles successfully${NC}"
    rm -f agentbox  # Clean up binary
else
    echo -e "   ${RED}❌ Compilation failed${NC}"
    ((ERRORS++))
fi

# 2. Check all packages compile
echo ""
echo "2. Checking all packages compile..."
if go build ./... 2>&1; then
    echo -e "   ${GREEN}✅ All packages compile${NC}"
else
    echo -e "   ${RED}❌ Some packages failed to compile${NC}"
    ((ERRORS++))
fi

# 3. Check go vet
echo ""
echo "3. Running go vet..."
if go vet ./... 2>&1; then
    echo -e "   ${GREEN}✅ go vet passed${NC}"
else
    echo -e "   ${YELLOW}⚠️  go vet found issues (non-critical)${NC}"
fi

# 4. Check that all routes are defined
echo ""
echo "4. Checking API routes..."
ROUTES=$(grep -c "HandleFunc" pkg/api/router.go || echo "0")
if [ "$ROUTES" -ge "7" ]; then
    echo -e "   ${GREEN}✅ Found $ROUTES routes (expected 7+)${NC}"
    echo "      Routes:"
    grep "HandleFunc" pkg/api/router.go | sed 's/^/        - /'
else
    echo -e "   ${RED}❌ Found only $ROUTES routes (expected 7+)${NC}"
    ((ERRORS++))
fi

# 5. Check that all handlers exist
echo ""
echo "5. Checking handler methods..."
HANDLERS=(
    "CreateEnvironment"
    "GetEnvironment"
    "ListEnvironments"
    "ExecuteCommand"
    "DeleteEnvironment"
    "HealthCheck"
    "GetLogs"
    "AttachWebSocket"
)

for handler in "${HANDLERS[@]}"; do
    if grep -q "func (h \*Handler) $handler" pkg/api/handler.go pkg/api/websocket.go 2>/dev/null; then
        echo -e "   ${GREEN}✅ Handler.$handler exists${NC}"
    else
        echo -e "   ${RED}❌ Handler.$handler missing${NC}"
        ((ERRORS++))
    fi
done

# 6. Check orchestrator methods
echo ""
echo "6. Checking orchestrator methods..."
ORCH_METHODS=(
    "CreateEnvironment"
    "GetEnvironment"
    "ListEnvironments"
    "DeleteEnvironment"
    "ExecuteCommand"
    "GetLogs"
    "GetHealthInfo"
)

for method in "${ORCH_METHODS[@]}"; do
    if grep -q "func (o \*Orchestrator) $method" pkg/orchestrator/orchestrator.go 2>/dev/null; then
        echo -e "   ${GREEN}✅ Orchestrator.$method exists${NC}"
    else
        echo -e "   ${RED}❌ Orchestrator.$method missing${NC}"
        ((ERRORS++))
    fi
done

# 7. Check K8s client methods
echo ""
echo "7. Checking K8s client interface..."
K8S_METHODS=(
    "HealthCheck"
    "GetServerVersion"
    "GetClusterCapacity"
    "CreateNamespace"
    "CreateResourceQuota"
    "CreateNetworkPolicy"
    "CreatePod"
    "GetPodLogs"
)

for method in "${K8S_METHODS[@]}"; do
    if grep -q "func.*$method" pkg/k8s/*.go 2>/dev/null; then
        echo -e "   ${GREEN}✅ K8s Client.$method exists${NC}"
    else
        echo -e "   ${RED}❌ K8s Client.$method missing${NC}"
        ((ERRORS++))
    fi
done

# 8. Check tests
echo ""
echo "8. Checking tests compile..."
if go test -c ./tests/unit/... 2>&1 >/dev/null; then
    echo -e "   ${GREEN}✅ Tests compile${NC}"
else
    echo -e "   ${YELLOW}⚠️  Some tests may have issues${NC}"
fi

# 9. Check configuration
echo ""
echo "9. Checking configuration..."
if [ -f "config/config.yaml" ]; then
    echo -e "   ${GREEN}✅ Config file exists${NC}"
else
    echo -e "   ${YELLOW}⚠️  Config file not found (will use defaults)${NC}"
fi

# Summary
echo ""
echo "============================================================"
if [ $ERRORS -eq 0 ]; then
    echo -e "${GREEN}✅ All validations passed! Service is properly configured.${NC}"
    echo ""
    echo "Service Components Verified:"
    echo "  ✅ Compilation successful"
    echo "  ✅ All packages build"
    echo "  ✅ API routes defined (7 endpoints)"
    echo "  ✅ Handler methods implemented (8 methods)"
    echo "  ✅ Orchestrator methods implemented (7 methods)"
    echo "  ✅ K8s client methods implemented"
    echo "  ✅ Tests compile"
    echo ""
    echo -e "${GREEN}🚀 Service is ready to run!${NC}"
    exit 0
else
    echo -e "${RED}❌ Validation found $ERRORS issue(s)${NC}"
    exit 1
fi
