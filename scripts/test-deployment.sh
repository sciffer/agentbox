#!/bin/bash

# AgentBox Deployment Test Script
# Tests Docker image and Helm chart deployment

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

ERRORS=0
WARNINGS=0

echo "🧪 Testing AgentBox Deployment"
echo "================================"
echo ""

# Test 1: Docker image build
echo "1. Testing Docker image build..."
if docker build -t agentbox:test -f Dockerfile . > /tmp/docker-build.log 2>&1; then
    echo -e "   ${GREEN}✅ Docker image builds successfully${NC}"
    docker images agentbox:test --format "   Image: {{.Repository}}:{{.Tag}} ({{.Size}})"
else
    echo -e "   ${RED}❌ Docker image build failed${NC}"
    cat /tmp/docker-build.log | tail -10
    ((ERRORS++))
fi
echo ""

# Test 2: Docker image structure
echo "2. Testing Docker image structure..."
if docker run --rm --entrypoint sh agentbox:test -c "test -f /app/agentbox" > /dev/null 2>&1; then
    echo -e "   ${GREEN}✅ Binary exists in container${NC}"
else
    echo -e "   ${RED}❌ Binary missing in container${NC}"
    ((ERRORS++))
fi

if docker run --rm --entrypoint sh agentbox:test -c "test -f /app/config/config.yaml" > /dev/null 2>&1; then
    echo -e "   ${GREEN}✅ Config file exists in container${NC}"
else
    echo -e "   ${RED}❌ Config file missing in container${NC}"
    ((ERRORS++))
fi

if docker run --rm --entrypoint sh agentbox:test -c "whoami" | grep -q "agentbox" > /dev/null 2>&1; then
    echo -e "   ${GREEN}✅ Container runs as non-root user${NC}"
else
    echo -e "   ${YELLOW}⚠️  Container user check failed (may still be secure)${NC}"
    ((WARNINGS++))
fi
echo ""

# Test 3: Docker image help command
echo "3. Testing Docker image help command..."
if docker run --rm agentbox:test --help 2>&1 | grep -q "config" > /dev/null 2>&1; then
    echo -e "   ${GREEN}✅ Application help command works${NC}"
else
    echo -e "   ${YELLOW}⚠️  Help command check inconclusive${NC}"
    ((WARNINGS++))
fi
echo ""

# Test 4: Helm chart lint
echo "4. Testing Helm chart lint..."
if helm lint ./helm/agentbox > /tmp/helm-lint.log 2>&1; then
    echo -e "   ${GREEN}✅ Helm chart lints successfully${NC}"
else
    echo -e "   ${RED}❌ Helm chart lint failed${NC}"
    cat /tmp/helm-lint.log
    ((ERRORS++))
fi
echo ""

# Test 5: Helm template rendering
echo "5. Testing Helm template rendering..."
if helm template agentbox ./helm/agentbox > /tmp/helm-template.yaml 2>&1; then
    echo -e "   ${GREEN}✅ Helm templates render successfully${NC}"
    
    # Count resources
    RESOURCE_COUNT=$(grep -c "^kind:" /tmp/helm-template.yaml || echo "0")
    echo "   Generated $RESOURCE_COUNT Kubernetes resources"
    
    # Check for required resources
    REQUIRED_RESOURCES=("Deployment" "Service" "ConfigMap" "ServiceAccount" "ClusterRole" "ClusterRoleBinding")
    for resource in "${REQUIRED_RESOURCES[@]}"; do
        if grep -q "kind: $resource" /tmp/helm-template.yaml; then
            echo -e "   ${GREEN}✅ $resource found${NC}"
        else
            echo -e "   ${RED}❌ $resource missing${NC}"
            ((ERRORS++))
        fi
    done
else
    echo -e "   ${RED}❌ Helm template rendering failed${NC}"
    ((ERRORS++))
fi
echo ""

# Test 6: Kubernetes manifest validation (if kubectl available)
echo "6. Testing Kubernetes manifest validation..."
if command -v kubectl > /dev/null 2>&1; then
    if kubectl apply --dry-run=client -f /tmp/helm-template.yaml > /tmp/kubectl-validate.log 2>&1; then
        echo -e "   ${GREEN}✅ Kubernetes manifests are valid${NC}"
    else
        echo -e "   ${YELLOW}⚠️  Kubernetes validation issues (may need cluster context)${NC}"
        cat /tmp/kubectl-validate.log | tail -5
        ((WARNINGS++))
    fi
else
    echo -e "   ${YELLOW}⚠️  kubectl not available, skipping manifest validation${NC}"
    ((WARNINGS++))
fi
echo ""

# Test 7: Check for security best practices
echo "7. Testing security configurations..."
if grep -q "runAsNonRoot: true" /tmp/helm-template.yaml; then
    echo -e "   ${GREEN}✅ Non-root user configured${NC}"
else
    echo -e "   ${YELLOW}⚠️  Non-root user not explicitly set${NC}"
    ((WARNINGS++))
fi

if grep -q "allowPrivilegeEscalation: false" /tmp/helm-template.yaml; then
    echo -e "   ${GREEN}✅ Privilege escalation disabled${NC}"
else
    echo -e "   ${YELLOW}⚠️  Privilege escalation not explicitly disabled${NC}"
    ((WARNINGS++))
fi

if grep -q "readOnlyRootFilesystem" /tmp/helm-template.yaml; then
    echo -e "   ${GREEN}✅ Read-only root filesystem considered${NC}"
else
    echo -e "   ${YELLOW}⚠️  Read-only root filesystem not set (may be intentional)${NC}"
    ((WARNINGS++))
fi
echo ""

# Test 8: Check health probes
echo "8. Testing health probe configuration..."
if grep -q "livenessProbe:" /tmp/helm-template.yaml && grep -q "readinessProbe:" /tmp/helm-template.yaml; then
    echo -e "   ${GREEN}✅ Health probes configured${NC}"
else
    echo -e "   ${RED}❌ Health probes missing${NC}"
    ((ERRORS++))
fi
echo ""

# Test 9: Check RBAC
echo "9. Testing RBAC configuration..."
if grep -q "kind: ClusterRole" /tmp/helm-template.yaml && grep -q "kind: ClusterRoleBinding" /tmp/helm-template.yaml; then
    echo -e "   ${GREEN}✅ RBAC resources configured${NC}"
    
    # Check for required permissions
    if grep -A 30 "kind: ClusterRole" /tmp/helm-template.yaml | grep -q "namespaces"; then
        echo -e "   ${GREEN}✅ Namespace permissions present${NC}"
    else
        echo -e "   ${RED}❌ Namespace permissions missing${NC}"
        ((ERRORS++))
    fi
else
    echo -e "   ${RED}❌ RBAC resources missing${NC}"
    ((ERRORS++))
fi
echo ""

# Summary
echo "================================"
if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
    echo -e "${GREEN}✅ All tests passed!${NC}"
    echo ""
    echo "Deployment is ready:"
    echo "  • Docker image: agentbox:test"
    echo "  • Helm chart: ./helm/agentbox"
    echo ""
    echo "Next steps:"
    echo "  1. Tag and push image: docker tag agentbox:test your-registry/agentbox:1.0.0"
    echo "  2. Install chart: helm install agentbox ./helm/agentbox"
    exit 0
elif [ $ERRORS -eq 0 ]; then
    echo -e "${YELLOW}⚠️  Tests passed with $WARNINGS warning(s)${NC}"
    echo ""
    echo "Deployment is functional but review warnings above."
    exit 0
else
    echo -e "${RED}❌ Tests failed with $ERRORS error(s) and $WARNINGS warning(s)${NC}"
    exit 1
fi
