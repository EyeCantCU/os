# Claude Code Knowledge Base

This document contains learnings and best practices discovered while working with this repository.

## Package Test Best Practices

### CRITICAL: Never ignore errors
- **NEVER** use `|| true` to suppress failures
- **NEVER** redirect errors to `/dev/null` without handling them
- **NEVER** use constructs that mask failures
- Tests should fail loudly and clearly when something goes wrong

**Bad patterns to avoid:**
```bash
# DON'T DO THIS - hides failures
some_command || true

# DON'T DO THIS - ignores errors
some_command 2>/dev/null || echo "continuing anyway"

# DON'T DO THIS - continues despite failure
if ! some_command; then
  echo "Oh well, moving on..."
fi
```

### 1. Use `set -euo pipefail` at the start of test scripts
```bash
set -euo pipefail
```
- `-e`: Exit immediately if any command fails
- `-u`: Treat unset variables as an error
- `-o pipefail`: Fail if any command in a pipeline fails (not just the last one)

This ensures tests fail fast and don't mask errors.

### 2. Use `jq` for JSON parsing instead of `grep`/`tail`/`head`
**Bad pattern:**
```bash
RESPONSE=$(curl -k -s -w "\n%{http_code}" "https://api.example.com/endpoint")
HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | head -n -1)
if [ "$HTTP_CODE" != "200" ]; then
  echo "Failed with HTTP $HTTP_CODE"; exit 1
fi
if ! echo "$BODY" | grep -q '"expected_field"'; then
  echo "Missing expected field"; exit 1
fi
```

**Good pattern:**
```bash
# Use curl -f to fail on HTTP errors
RESPONSE=$(curl -k -s -f "https://api.example.com/endpoint")
# Use jq -e to validate and exit on false conditions
echo "$RESPONSE" | jq -e '.expected_field == "expected_value"'
```

### 3. Use `curl -f` flag for API tests
The `-f` flag makes curl return non-zero exit code on HTTP errors (4xx, 5xx), eliminating the need to manually check status codes. Combined with `set -e`, this provides automatic failure handling.

### 4. Simplify validation with `jq -e`
The `-e` flag makes jq exit with status 1 if the expression evaluates to false or null:
```bash
# Single field validation
echo "$RESPONSE" | jq -e '.userName == "testuser123"'

# Multiple fields validation
echo "$RESPONSE" | jq -e '.issuer and .token_endpoint and .jwks_uri'

# Array contains check
echo "$RESPONSE" | jq -e '.emails | contains(["test@example.com"])'

# Length validation
echo "$RESPONSE" | jq -e '.keys | length > 0'
```