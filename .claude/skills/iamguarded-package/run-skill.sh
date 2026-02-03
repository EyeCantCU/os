#!/usr/bin/env bash
set -euo pipefail

#
# Wrapper script for iamguarded-package skill
#
# Usage: ./run-skill.sh [APPLICATION_NAME]
#
# This script:
# 1. Installs Python dependencies with uv
# 2. Registers the MCP server in local scope
# 3. Launches Claude with the skill (optionally with application name)
# 4. Cleans up the MCP registration on exit
#

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVER_PATH="${SCRIPT_DIR}/server.py"
REQUIREMENTS_PATH="${SCRIPT_DIR}/requirements.txt"
MCP_SERVER_NAME="iamguarded-package"

# Get the enterprise-packages root (3 levels up from script dir)
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# Get optional application name argument
APPLICATION_NAME="${1:-}"

# Cleanup function to remove MCP server
cleanup() {
  echo "Cleaning up MCP server registration..." >&2
  claude mcp remove "${MCP_SERVER_NAME}" --scope local 2>/dev/null || true
}

# Set trap to cleanup on exit
trap cleanup EXIT INT TERM

# Validate we're in enterprise-packages repository
if [[ ! -e "${REPO_ROOT}/.git" ]]; then
  echo "ERROR: Not in a git repository. Expected enterprise-packages root at ${REPO_ROOT}" >&2
  exit 1
fi

# Check if uv is installed
if ! command -v uv &> /dev/null; then
  echo "ERROR: uv is not installed. Install it with: curl -LsSf https://astral.sh/uv/install.sh | sh" >&2
  exit 1
fi

# Check if server.py exists
if [[ ! -f "${SERVER_PATH}" ]]; then
  echo "ERROR: MCP server not found at ${SERVER_PATH}" >&2
  exit 1
fi

echo "Working directory: ${REPO_ROOT}" >&2

# Install Python dependencies with uv
echo "Installing Python dependencies with uv..." >&2
# cd "${SCRIPT_DIR}"
# # uv pip install -q -r "${REQUIREMENTS_PATH}"

# Register the MCP server in local scope using uv run
# Use bash wrapper to cd to repo root before running server
echo "Registering MCP server (local scope)..." >&2
claude mcp add --transport stdio --scope local "${MCP_SERVER_NAME}" -- \
  bash -c "cd '${REPO_ROOT}' && uv run --with-requirements '${REQUIREMENTS_PATH}' --with mcp --with pyyaml '${SERVER_PATH}'"

# Launch Claude with the skill
if [[ -n "${APPLICATION_NAME}" ]]; then
  echo "Launching Claude with ${MCP_SERVER_NAME} skill for application: ${APPLICATION_NAME}..." >&2
  echo "" >&2

  # Run Claude with initial prompt to invoke the skill with the application name
  claude "Use the /iamguarded-package skill to create an iamguarded compat package for ${APPLICATION_NAME}"
else
  echo "Launching Claude with ${MCP_SERVER_NAME} skill..." >&2
  echo "" >&2

  # Run Claude in interactive mode
  claude
fi

# Cleanup happens automatically via trap
