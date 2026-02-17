#!/usr/bin/env bash
#
# Wrapper script for authentik-patch skill
# Manages MCP server lifecycle and launches Claude with the skill
#

set -euo pipefail

# Determine the skill directory
SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVER_PATH="${SKILL_DIR}/server.py"
REQUIREMENTS_PATH="${SKILL_DIR}/requirements.txt"
MCP_SERVER_NAME="authentik-patch"

# Optional version argument
VERSION_ARG="${1:-}"

# Cleanup function to remove MCP server registration
cleanup() {
    echo "Cleaning up MCP server registration..." >&2
    claude mcp remove "${MCP_SERVER_NAME}" --scope local 2>/dev/null || true
}

# Register cleanup on exit
trap cleanup EXIT INT TERM

# Check for required tools
command -v git >/dev/null 2>&1 || {
    echo "ERROR: git is not installed" >&2
    echo "Please install git and try again" >&2
    exit 1
}

command -v rg >/dev/null 2>&1 || {
    echo "ERROR: ripgrep (rg) is not installed" >&2
    echo "Install with: brew install ripgrep" >&2
    exit 1
}

command -v uv >/dev/null 2>&1 || {
    echo "ERROR: uv is not installed" >&2
    echo "Install with: curl -LsSf https://astral.sh/uv/install.sh | sh" >&2
    exit 1
}

# Validate working directory
if [[ ! -d "enterprise-packages" && ! -d ".git" ]]; then
    echo "ERROR: Must be run from enterprise-packages repository root" >&2
    echo "Current directory: $(pwd)" >&2
    echo "Expected: /Users/matthew.ramirez/work/stereo" >&2
    exit 1
fi

# Get absolute path to repository root
REPO_ROOT="$(pwd)"

echo "Starting authentik-patch skill..." >&2
echo "MCP server: ${SERVER_PATH}" >&2
echo "Working directory: ${REPO_ROOT}" >&2

# Register MCP server (local scope only)
echo "Registering MCP server..." >&2
claude mcp add --transport stdio --scope local "${MCP_SERVER_NAME}" -- \
    bash -c "cd '${REPO_ROOT}' && uv run --with-requirements '${REQUIREMENTS_PATH}' --with mcp --with pyyaml --with ruamel.yaml '${SERVER_PATH}'"

# Build the initial prompt
if [[ -n "${VERSION_ARG}" ]]; then
    INITIAL_PROMPT="Use the /authentik-patch skill to update authentik to version ${VERSION_ARG}"
    echo "Launching Claude with version: ${VERSION_ARG}" >&2
else
    INITIAL_PROMPT=""
    echo "Launching Claude in interactive mode..." >&2
    echo "Use /authentik-patch to start the skill" >&2
fi

# Launch Claude
if [[ -n "${INITIAL_PROMPT}" ]]; then
    claude "${INITIAL_PROMPT}"
else
    claude
fi

# Cleanup will be called automatically via trap
