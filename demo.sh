#!/bin/bash

# Configuration
BASE_URL="http://localhost:8081"
TIMESTAMP=$(date +%s)

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}=== API Control Plane Demo ===${NC}"
echo "Target: $BASE_URL"

# 1. Health Check
echo -e "\n${GREEN}1. Infrastructure Health Tests${NC}"
echo "Checking Liveness (/health)..."
curl -s "$BASE_URL/health" && echo " [OK]"
echo "Checking Readiness (/ready - Redis Connectivity)..."
curl -s "$BASE_URL/ready" && echo " [OK]"

# 2. Security Test
echo -e "\n${GREEN}2. Security Enforcement Test${NC}"
echo "Attempting to reload policy WITHOUT credentials..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "X-Timestamp: $TIMESTAMP" "$BASE_URL/api/admin/reload")
if [ "$HTTP_CODE" == "401" ]; then
    echo -e "Result: ${GREEN}BLOCKED (401 Unauthorized)${NC} - Policy Engine working correctly."
else
    echo -e "Result: ${RED}FAILED ($HTTP_CODE)${NC}"
fi

# 3. Use Test Door to get Admin Key
echo -e "\n${GREEN}3. bootstrapping Admin Credentials${NC}"
ADMIN_KEY=$(curl -s -H "X-Timestamp: $TIMESTAMP" "$BASE_URL/api/test/generate-key?user_id=admin")
echo -e "Generated Temporary Admin Key: ${GREEN}${ADMIN_KEY:0:10}...${NC}"

# 4. Create User Key
echo -e "\n${GREEN}4. Provisioning New User Identity${NC}"
TIMESTAMP=$(date +%s)
RESP=$(curl -s -H "X-API-Key: $ADMIN_KEY" -H "X-Timestamp: $TIMESTAMP" -H "Content-Type: application/json" -d '{"user_id": "demo-user", "name": "demo-key"}' "$BASE_URL/api/admin/keys/create")
USER_KEY=$(echo $RESP | jq -r '.api_key')
echo -e "Created Key for 'demo-user': ${GREEN}${USER_KEY:0:10}...${NC}"

# 5. Verify Access
echo -e "\n${GREEN}5. Verifying Access with New Key (/api/whoami)${NC}"
TIMESTAMP=$(date +%s)
WHOAMI=$(curl -s -H "X-API-Key: $USER_KEY" -H "X-Timestamp: $TIMESTAMP" "$BASE_URL/api/whoami")
echo "Server Response: $WHOAMI"

# 6. Rate Limit Test
echo -e "\n${GREEN}6. Rate Limiting Stress Test (Burst)${NC}"
echo "Sending 5 rapid requests to public endpoint..."
for i in {1..5}; do
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" -H "X-Timestamp: $TIMESTAMP" "$BASE_URL/api/public/hello")
    if [ "$STATUS" == "200" ]; then
        echo -n "✅ "
    else
        echo -n "❌ ($STATUS) "
    fi
done
echo -e "\n(All requests should pass if within burst limit)"

echo -e "\n${GREEN}=== Demo Complete ===${NC}"
