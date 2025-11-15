#!/bin/bash
#
# Integration Test Runner
# Starts the test environment and runs the integration test suite
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================"
echo "  Home Automation Integration Tests"
echo "========================================"
echo ""

# Change to integration test directory
cd "$(dirname "$0")"

# Create log directory
mkdir -p test-logs

echo -e "${YELLOW}→ Building Docker images...${NC}"
docker-compose -f docker-compose.test.yml build --quiet

echo -e "${YELLOW}→ Starting test environment...${NC}"
docker-compose -f docker-compose.test.yml up -d

echo -e "${YELLOW}→ Waiting for Mock HA service to be ready...${NC}"
timeout 60s bash -c '
  until docker-compose -f docker-compose.test.yml exec -T mockha curl -f http://localhost:8123/test/health 2>/dev/null; do
    echo "  Waiting for Mock HA..."
    sleep 2
  done
' || {
  echo -e "${RED}✗ Mock HA failed to start${NC}"
  docker-compose -f docker-compose.test.yml logs mockha
  docker-compose -f docker-compose.test.yml down -v
  exit 1
}
echo -e "${GREEN}✓ Mock HA is ready${NC}"

echo -e "${YELLOW}→ Waiting for homeautomation service to be ready...${NC}"
timeout 90s bash -c '
  until docker-compose -f docker-compose.test.yml exec -T homeautomation wget -q --spider http://localhost:8080/health 2>/dev/null; do
    echo "  Waiting for homeautomation service..."
    sleep 3
  done
' || {
  echo -e "${RED}✗ Homeautomation service failed to start${NC}"
  docker-compose -f docker-compose.test.yml logs homeautomation
  docker-compose -f docker-compose.test.yml down -v
  exit 1
}
echo -e "${GREEN}✓ Homeautomation service is ready${NC}"

echo ""
echo -e "${YELLOW}→ Running integration tests...${NC}"
echo ""

# Run the tests inside the container
# NOTE: This assumes Go tests are compiled into the homeautomation container
# In a real implementation, you might run tests from a separate test container
docker-compose -f docker-compose.test.yml exec -T homeautomation \
  go test -v ./tests/integration/tests/... -timeout 10m 2>&1 | tee test-logs/test-output.log

TEST_EXIT_CODE=${PIPESTATUS[0]}

echo ""
echo -e "${YELLOW}→ Collecting logs...${NC}"
docker-compose -f docker-compose.test.yml logs homeautomation > test-logs/homeautomation.log 2>&1
docker-compose -f docker-compose.test.yml logs mockha > test-logs/mockha.log 2>&1

echo -e "${YELLOW}→ Shutting down test environment...${NC}"
docker-compose -f docker-compose.test.yml down -v

echo ""
echo "========================================"
if [ $TEST_EXIT_CODE -eq 0 ]; then
  echo -e "${GREEN}✓ All tests passed!${NC}"
  echo "========================================"
  exit 0
else
  echo -e "${RED}✗ Tests failed!${NC}"
  echo "========================================"
  echo ""
  echo "Check test-logs/ directory for details:"
  echo "  - test-output.log: Test execution output"
  echo "  - homeautomation.log: Application logs"
  echo "  - mockha.log: Mock HA service logs"
  exit $TEST_EXIT_CODE
fi
