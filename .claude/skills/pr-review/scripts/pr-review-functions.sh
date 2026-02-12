#!/bin/bash

# PR Review Functions - Shell utilities for Claude Code PR review skill
# This script provides individual functions that can be called by Claude Code
# to orchestrate PR reviews for the enterprise-packages repository.

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored messages
info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

# Constants for enterprise-packages repo
GITHUB_REPO="chainguard-dev/enterprise-packages"
REPO_URL="https://github.com/$GITHUB_REPO.git"

# Function: setup_temp_dir
# Creates a temporary directory for the PR review
# Outputs the temp directory path to stdout
# A temporary directory is used to keep the workspace isolated
setup_temp_dir() {
    local temp_dir
    temp_dir=$(mktemp -d -t pr-review-XXXXXX)
    echo "$temp_dir"
}

# Function: clone_repo
# Clones the enterprise-packages repository into the specified directory
# Args: $1 - target directory path
clone_repo() {
    local target_dir="$1"

    if [ -z "$target_dir" ]; then
        error "Target directory not specified"
        return 1
    fi

    info "Cloning $GITHUB_REPO into $target_dir..."

    if [ -d "$target_dir/.git" ]; then
        warning "Repository already exists at $target_dir"
        return 0
    fi

    git clone "$REPO_URL" "$target_dir"
    success "Repository cloned successfully"
}

# Function: checkout_pr
# Checks out a specific PR in the repository
# Args: $1 - PR number
#       $2 - repository directory path
checkout_pr() {
    local pr_number="$1"
    local repo_dir="$2"

    if [ -z "$pr_number" ]; then
        error "PR number not specified"
        return 1
    fi

    if [ -z "$repo_dir" ]; then
        error "Repository directory not specified"
        return 1
    fi

    if ! [[ "$pr_number" =~ ^[0-9]+$ ]]; then
        error "PR number must be numeric"
        return 1
    fi

    info "Checking out PR #$pr_number..."

    cd "$repo_dir"

    # Fetch the PR
    info "Fetching PR #$pr_number from $GITHUB_REPO..."
    git fetch origin "pull/$pr_number/head:pr-$pr_number"

    # Checkout the PR branch
    git checkout "pr-$pr_number"

    success "Checked out PR #$pr_number successfully"
}

# Function: cleanup_temp_dir
# Removes the temporary directory
# Args: $1 - directory path to remove
cleanup_temp_dir() {
    local dir_path="$1"

    if [ -z "$dir_path" ]; then
        error "Directory path not specified"
        return 1
    fi

    if [ ! -d "$dir_path" ]; then
        warning "Directory does not exist: $dir_path"
        return 0
    fi

    info "Cleaning up temporary directory: $dir_path"
    rm -rf "$dir_path"
    success "Cleanup complete"
}

# Function: get_pr_info
# Retrieves PR information using gh CLI
# Args: $1 - PR number
get_pr_info() {
    local pr_number="$1"

    if [ -z "$pr_number" ]; then
        error "PR number not specified"
        return 1
    fi

    if ! command -v gh &> /dev/null; then
        error "gh CLI is not installed"
        return 1
    fi

    info "Fetching PR #$pr_number information..."
    gh pr view "$pr_number" -R "$GITHUB_REPO" --json number,title,author,body,headRefName
}

# Function: check_requirements
# Verifies that all required commands are available
check_requirements() {
    local missing_commands=()

    if ! command -v git &> /dev/null; then
        missing_commands+=("git")
    fi

    if ! command -v gh &> /dev/null; then
        missing_commands+=("gh")
    fi

    if [ ${#missing_commands[@]} -ne 0 ]; then
        error "Missing required commands: ${missing_commands[*]}"
        error "Please install the missing commands and try again."
        return 1
    fi

    success "All required commands are available"
}

# Main function dispatcher
# Allows calling functions by name: ./pr-review-functions.sh function_name args...
if [ $# -gt 0 ]; then
    function_name="$1"
    shift

    if declare -f "$function_name" > /dev/null; then
        "$function_name" "$@"
    else
        error "Unknown function: $function_name"
        echo "Usage examples:"
        echo "  ./pr-review-functions.sh check_requirements"
        echo "  ./pr-review-functions.sh setup_temp_dir"
        echo "  ./pr-review-functions.sh clone_repo /tmp/pr-review-dir"
        echo ""
        echo "Available functions:"
        echo "  setup_temp_dir"
        echo "  clone_repo <target_dir>"
        echo "  checkout_pr <pr_number> <repo_dir>"
        echo "  cleanup_temp_dir <dir_path>"
        echo "  get_pr_info <pr_number>"
        echo "  check_requirements"
        exit 1
    fi
fi
