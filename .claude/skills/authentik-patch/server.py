#!/usr/bin/env python3
"""
Authentik Patch MCP Server

Provides tools for automating authentik enterprise patch updates.
"""

import os
import subprocess
import tempfile
import shutil
import json
import re
from pathlib import Path
from typing import Optional, Dict, Any, List, Tuple
import yaml

try:
    from ruamel.yaml import YAML
    has_ruamel = True
except ImportError:
    has_ruamel = False

from mcp.server import Server
from mcp.types import Tool, TextContent

# Global state for managing temporary directories
_temp_repos: Dict[str, str] = {}
_analysis_cache: Dict[str, Any] = {}

app = Server("authentik-patch")


def validate_working_directory() -> Optional[str]:
    """Validate that we're in the enterprise-packages repository root."""
    if not Path("enterprise-packages").exists() and not Path(".git").exists():
        return "ERROR: Must be run from enterprise-packages repository root"

    # Check if this is actually a git repo
    returncode, stdout, stderr = run_command([
        "git", "rev-parse", "--git-dir"
    ])
    if returncode != 0:
        return f"ERROR: {stderr}"
    return None


def run_command(cmd: list[str], cwd: Optional[str] = None) -> tuple[int, str, str]:
    """Run a command and return exit code, stdout, stderr."""
    try:
        result = subprocess.run(
            cmd,
            cwd=cwd,
            capture_output=True,
            text=True
        )
        return result.returncode, result.stdout, result.stderr
    except Exception as e:
        return 1, "", str(e)


@app.list_tools()
async def list_tools() -> list[Tool]:
    """List available tools."""
    return [
        Tool(
            name="checkout_authentik",
            description="Clone authentik repository and checkout a specific version. Returns repo path and commit hash. Maintains state to avoid re-cloning.",
            inputSchema={
                "type": "object",
                "properties": {
                    "version": {
                        "type": "string",
                        "description": "Version to checkout (e.g., '2025.12.3')"
                    }
                },
                "required": ["version"]
            }
        ),
        Tool(
            name="list_upstream_versions",
            description="List recent authentik version tags from upstream repository.",
            inputSchema={
                "type": "object",
                "properties": {
                    "limit": {
                        "type": "integer",
                        "description": "Maximum number of versions to return (default: 10)",
                        "default": 10
                    }
                },
                "required": []
            }
        ),
        Tool(
            name="find_enterprise_refs",
            description="Find all authentik.enterprise references in Python files using ripgrep. Returns list of files with line numbers and matches.",
            inputSchema={
                "type": "object",
                "properties": {
                    "repo_path": {
                        "type": "string",
                        "description": "Path to authentik repository"
                    }
                },
                "required": ["repo_path"]
            }
        ),
        Tool(
            name="analyze_patch_coverage",
            description="Compare existing patches with current enterprise references to identify new, removed, or unchanged files.",
            inputSchema={
                "type": "object",
                "properties": {
                    "repo_path": {
                        "type": "string",
                        "description": "Path to authentik repository"
                    },
                    "patch_dir": {
                        "type": "string",
                        "description": "Path to directory containing patch files (default: enterprise-packages/authentik/)",
                        "default": "enterprise-packages/authentik/"
                    }
                },
                "required": ["repo_path"]
            }
        ),
        Tool(
            name="generate_patch_template",
            description="Generate a patch template for a file showing enterprise references with context. Helps guide manual patch creation.",
            inputSchema={
                "type": "object",
                "properties": {
                    "file_path": {
                        "type": "string",
                        "description": "Relative path to file in authentik repo (e.g., 'authentik/core/api/users.py')"
                    },
                    "repo_path": {
                        "type": "string",
                        "description": "Path to authentik repository"
                    }
                },
                "required": ["file_path", "repo_path"]
            }
        ),
        Tool(
            name="validate_patches",
            description="Validate that all patch files apply cleanly to the checked out version.",
            inputSchema={
                "type": "object",
                "properties": {
                    "repo_path": {
                        "type": "string",
                        "description": "Path to authentik repository"
                    },
                    "patch_dir": {
                        "type": "string",
                        "description": "Path to directory containing patch files (default: enterprise-packages/authentik/)",
                        "default": "enterprise-packages/authentik/"
                    }
                },
                "required": ["repo_path"]
            }
        ),
        Tool(
            name="sync_patches",
            description="Sync patch files from one package directory to another (e.g., authentik/ to authentik-fips/). Copies all patches, reports any pre-existing differences that were overwritten, and detects stale patches in the target.",
            inputSchema={
                "type": "object",
                "properties": {
                    "source_dir": {
                        "type": "string",
                        "description": "Source patch directory (default: enterprise-packages/authentik/)",
                        "default": "enterprise-packages/authentik/"
                    },
                    "target_dir": {
                        "type": "string",
                        "description": "Target patch directory (default: enterprise-packages/authentik-fips/)",
                        "default": "enterprise-packages/authentik-fips/"
                    },
                    "dry_run": {
                        "type": "boolean",
                        "description": "If true, report what would be done without making changes (default: false)",
                        "default": False
                    }
                },
                "required": []
            }
        ),
        Tool(
            name="update_package_yaml",
            description="Update package YAML file with new version, commit hash, and patch list. Preserves formatting and comments.",
            inputSchema={
                "type": "object",
                "properties": {
                    "package_name": {
                        "type": "string",
                        "description": "Package name ('authentik' or 'authentik-fips')"
                    },
                    "version": {
                        "type": "string",
                        "description": "New version number (e.g., '2025.12.3')"
                    },
                    "commit": {
                        "type": "string",
                        "description": "Git commit hash"
                    },
                    "patches": {
                        "type": "array",
                        "items": {"type": "string"},
                        "description": "List of patch filenames (optional - if not provided, patches won't be updated)"
                    }
                },
                "required": ["package_name", "version", "commit"]
            }
        ),
        Tool(
            name="cleanup",
            description="Remove a temporary directory created by checkout_authentik.",
            inputSchema={
                "type": "object",
                "properties": {
                    "temp_dir": {
                        "type": "string",
                        "description": "Path to the temporary directory to remove"
                    }
                },
                "required": ["temp_dir"]
            }
        ),
    ]


@app.call_tool()
async def call_tool(name: str, arguments: Any) -> list[TextContent]:
    """Handle tool calls."""

    # Validate working directory for all tools except list_upstream_versions
    if name != "list_upstream_versions":
        error = validate_working_directory()
        if error:
            return [TextContent(type="text", text=error)]

    if name == "checkout_authentik":
        return await checkout_authentik(arguments["version"])
    elif name == "list_upstream_versions":
        limit = arguments.get("limit", 10)
        return await list_upstream_versions(limit)
    elif name == "find_enterprise_refs":
        return await find_enterprise_refs(arguments["repo_path"])
    elif name == "analyze_patch_coverage":
        patch_dir = arguments.get("patch_dir", "enterprise-packages/authentik/")
        return await analyze_patch_coverage(arguments["repo_path"], patch_dir)
    elif name == "generate_patch_template":
        return await generate_patch_template(
            arguments["file_path"],
            arguments["repo_path"]
        )
    elif name == "validate_patches":
        patch_dir = arguments.get("patch_dir", "enterprise-packages/authentik/")
        return await validate_patches(arguments["repo_path"], patch_dir)
    elif name == "sync_patches":
        source_dir = arguments.get("source_dir", "enterprise-packages/authentik/")
        target_dir = arguments.get("target_dir", "enterprise-packages/authentik-fips/")
        dry_run = arguments.get("dry_run", False)
        return await sync_patches(source_dir, target_dir, dry_run)
    elif name == "update_package_yaml":
        patches = arguments.get("patches")
        return await update_package_yaml(
            arguments["package_name"],
            arguments["version"],
            arguments["commit"],
            patches
        )
    elif name == "cleanup":
        return await cleanup(arguments["temp_dir"])
    else:
        return [TextContent(type="text", text=f"ERROR: Unknown tool: {name}")]


async def checkout_authentik(version: str) -> list[TextContent]:
    """Clone authentik repository and checkout a specific version."""

    repo_key = f"authentik-{version}"

    # Check if already cloned
    if repo_key in _temp_repos:
        repo_path = _temp_repos[repo_key]
        if Path(repo_path).exists():
            # Get commit hash
            returncode, commit_hash, stderr = run_command(
                ["git", "rev-parse", "HEAD"],
                cwd=repo_path
            )
            if returncode == 0:
                return [TextContent(
                    type="text",
                    text=f"Repository already cloned at: {repo_path}\nCommit: {commit_hash.strip()}"
                )]

    # Create temp directory
    temp_base = tempfile.mkdtemp(prefix="authentik-patch-")
    repo_path = Path(temp_base) / "authentik"

    # Clone the repository
    repo_url = "https://github.com/goauthentik/authentik.git"
    returncode, stdout, stderr = run_command([
        "git", "clone", "-q", "--depth", "50", repo_url, str(repo_path)
    ])

    if returncode != 0:
        shutil.rmtree(temp_base, ignore_errors=True)
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to clone authentik repository\n{stderr}"
        )]

    # Checkout the specific version tag
    tag = f"version/{version}"
    returncode, stdout, stderr = run_command(
        ["git", "checkout", tag],
        cwd=str(repo_path)
    )

    if returncode != 0:
        shutil.rmtree(temp_base, ignore_errors=True)
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to checkout {tag}\n{stderr}\nMake sure the version exists."
        )]

    # Get commit hash
    returncode, commit_hash, stderr = run_command(
        ["git", "rev-parse", "HEAD"],
        cwd=str(repo_path)
    )

    if returncode != 0:
        shutil.rmtree(temp_base, ignore_errors=True)
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to get commit hash\n{stderr}"
        )]

    # Store the path
    _temp_repos[repo_key] = str(repo_path)

    return [TextContent(
        type="text",
        text=f"Repository path: {repo_path}\nCommit: {commit_hash.strip()}"
    )]


async def list_upstream_versions(limit: int = 10) -> list[TextContent]:
    """List recent authentik version tags from upstream."""

    # Fetch tags from remote
    repo_url = "https://github.com/goauthentik/authentik.git"
    returncode, stdout, stderr = run_command([
        "git", "ls-remote", "--tags", repo_url
    ])

    if returncode != 0:
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to fetch tags\n{stderr}"
        )]

    # Parse version tags
    versions = []
    for line in stdout.strip().split('\n'):
        if not line:
            continue
        match = re.search(r'refs/tags/version/([\d.]+)$', line)
        if match:
            versions.append(match.group(1))

    # Sort versions (newest first)
    def version_key(v):
        try:
            parts = [int(x) for x in v.split('.')]
            return parts
        except:
            return [0, 0, 0]

    versions.sort(key=version_key, reverse=True)

    # Limit results
    versions = versions[:limit]

    if not versions:
        return [TextContent(
            type="text",
            text="No version tags found"
        )]

    return [TextContent(
        type="text",
        text="Recent authentik versions:\n" + "\n".join(f"  - {v}" for v in versions)
    )]


async def find_enterprise_refs(repo_path: str) -> list[TextContent]:
    """Find all authentik.enterprise references using ripgrep."""

    repo = Path(repo_path)
    if not repo.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: Repository path does not exist: {repo_path}"
        )]

    # Run ripgrep to find enterprise references
    # NOTE: Use -g '*.py' instead of -t py because ripgrep 15.x has a bug
    # where combining -t (type filter) with -g '!' (negative glob) causes
    # the type override and glob override to conflict, resulting in zero
    # files being searched.
    returncode, stdout, stderr = run_command([
        "rg",
        "authentik\\.enterprise",
        "-g", "*.py",
        "-g", "!authentik/enterprise/**",
        "--json"
    ], cwd=str(repo))

    if returncode != 0 and returncode != 1:  # rg returns 1 if no matches
        return [TextContent(
            type="text",
            text=f"ERROR: ripgrep failed\n{stderr}"
        )]

    if not stdout:
        return [TextContent(
            type="text",
            text="No enterprise references found"
        )]

    # Parse JSON output and group by file
    file_matches = {}
    for line in stdout.strip().split('\n'):
        try:
            data = json.loads(line)
            if data.get('type') == 'match':
                path = data['data']['path']['text']
                line_num = data['data']['line_number']
                match_text = data['data']['lines']['text'].strip()

                if path not in file_matches:
                    file_matches[path] = []
                file_matches[path].append((line_num, match_text))
        except (json.JSONDecodeError, KeyError):
            continue

    # Cache results for later use
    _analysis_cache['enterprise_refs'] = file_matches

    # Format output
    if not file_matches:
        return [TextContent(
            type="text",
            text="No enterprise references found"
        )]

    output_lines = [f"Found enterprise references in {len(file_matches)} files:\n"]
    for file_path, matches in sorted(file_matches.items()):
        output_lines.append(f"\n{file_path} ({len(matches)} references):")
        for line_num, match_text in matches[:3]:  # Show first 3 matches per file
            output_lines.append(f"  Line {line_num}: {match_text[:80]}")
        if len(matches) > 3:
            output_lines.append(f"  ... and {len(matches) - 3} more")

    return [TextContent(type="text", text="\n".join(output_lines))]


async def analyze_patch_coverage(repo_path: str, patch_dir: str) -> list[TextContent]:
    """Compare existing patches with current enterprise references."""

    # First, get current enterprise references
    if 'enterprise_refs' not in _analysis_cache:
        await find_enterprise_refs(repo_path)

    current_refs = _analysis_cache.get('enterprise_refs', {})
    current_files = set(current_refs.keys())

    # Parse existing patches to find which files they cover
    patch_path = Path(patch_dir)
    if not patch_path.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: Patch directory does not exist: {patch_dir}"
        )]

    patched_files = set()
    patch_files = list(patch_path.glob("*.patch"))

    for patch_file in patch_files:
        try:
            with open(patch_file, 'r') as f:
                content = f.read()
                # Find all files mentioned in the patch
                for match in re.finditer(r'^(?:\+\+\+|---) [ab]/(.*?)(?:\s|$)', content, re.MULTILINE):
                    file_path = match.group(1)
                    patched_files.add(file_path)
        except Exception as e:
            continue

    # Analyze coverage
    new_files = current_files - patched_files
    removed_files = patched_files - current_files
    covered_files = current_files & patched_files

    # Format output
    output_lines = ["## Patch Coverage Analysis\n"]

    output_lines.append(f"**Current enterprise references**: {len(current_files)} files")
    output_lines.append(f"**Existing patches cover**: {len(covered_files)} files")
    output_lines.append(f"**New files** (not in patches): {len(new_files)}")
    output_lines.append(f"**Removed files** (in patches but no refs): {len(removed_files)}\n")

    if new_files:
        output_lines.append("### New Files Needing Patches:")
        for f in sorted(new_files):
            ref_count = len(current_refs[f])
            output_lines.append(f"  - {f} ({ref_count} references)")

    if removed_files:
        output_lines.append("\n### Files No Longer Have Enterprise Refs:")
        for f in sorted(removed_files):
            output_lines.append(f"  - {f}")

    if covered_files:
        output_lines.append(f"\n### Covered Files ({len(covered_files)}):")
        output_lines.append("  (These files are already patched and may need updates)")

    # Cache analysis for later use
    _analysis_cache['coverage'] = {
        'new_files': list(new_files),
        'removed_files': list(removed_files),
        'covered_files': list(covered_files)
    }

    return [TextContent(type="text", text="\n".join(output_lines))]


async def generate_patch_template(file_path: str, repo_path: str) -> list[TextContent]:
    """Generate a patch template for a file showing enterprise references."""

    repo = Path(repo_path)
    full_path = repo / file_path

    if not full_path.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: File does not exist: {file_path}"
        )]

    # Read the file
    try:
        with open(full_path, 'r') as f:
            lines = f.readlines()
    except Exception as e:
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to read file: {str(e)}"
        )]

    # Find lines with enterprise references
    matches = []
    for i, line in enumerate(lines, 1):
        if 'authentik.enterprise' in line:
            matches.append((i, line.rstrip()))

    if not matches:
        return [TextContent(
            type="text",
            text=f"No enterprise references found in {file_path}"
        )]

    # Generate template with context
    output_lines = [f"## Patch Template for {file_path}\n"]
    output_lines.append(f"Found {len(matches)} enterprise references:\n")

    for line_num, line_text in matches:
        # Show context (3 lines before and after)
        start_idx = max(0, line_num - 4)
        end_idx = min(len(lines), line_num + 3)

        output_lines.append(f"\n### Line {line_num}:")
        output_lines.append("```python")
        for i in range(start_idx, end_idx):
            prefix = ">>>" if i == line_num - 1 else "   "
            output_lines.append(f"{prefix} {lines[i].rstrip()}")
        output_lines.append("```")

    output_lines.append("\n## Common Patch Patterns:")
    output_lines.append("1. Comment out imports: `#from authentik.enterprise...`")
    output_lines.append("2. Replace license checks with None")
    output_lines.append("3. Comment out ConditionalInheritance calls")
    output_lines.append("4. Create empty stub classes where needed")
    output_lines.append("\nSee references/patch-patterns.md for detailed examples.")

    return [TextContent(type="text", text="\n".join(output_lines))]


async def validate_patches(repo_path: str, patch_dir: str) -> list[TextContent]:
    """Validate that all patches apply cleanly."""

    repo = Path(repo_path)
    patch_path = Path(patch_dir)

    if not repo.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: Repository path does not exist: {repo_path}"
        )]

    if not patch_path.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: Patch directory does not exist: {patch_dir}"
        )]

    # Find all patch files
    patch_files = sorted(patch_path.glob("*.patch"))

    if not patch_files:
        return [TextContent(
            type="text",
            text=f"No patch files found in {patch_dir}"
        )]

    # Test each patch
    results = []
    for patch_file in patch_files:
        returncode, stdout, stderr = run_command(
            ["git", "apply", "--check", str(patch_file)],
            cwd=str(repo)
        )

        if returncode == 0:
            results.append(f"✓ {patch_file.name}: OK")
        else:
            results.append(f"✗ {patch_file.name}: FAILED")
            if stderr:
                results.append(f"  Error: {stderr[:200]}")

    # Summary
    passed = sum(1 for r in results if r.startswith("✓"))
    failed = len(patch_files) - passed

    summary = f"## Patch Validation Results\n\n"
    summary += f"**Total**: {len(patch_files)} patches\n"
    summary += f"**Passed**: {passed}\n"
    summary += f"**Failed**: {failed}\n\n"
    summary += "\n".join(results)

    return [TextContent(type="text", text=summary)]


async def sync_patches(
    source_dir: str,
    target_dir: str,
    dry_run: bool = False
) -> list[TextContent]:
    """Sync patch files from one package directory to another."""

    source_path = Path(source_dir)
    target_path = Path(target_dir)

    if not source_path.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: Source directory does not exist: {source_dir}"
        )]

    if not target_path.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: Target directory does not exist: {target_dir}"
        )]

    source_patches = sorted(source_path.glob("*.patch"))
    if not source_patches:
        return [TextContent(
            type="text",
            text=f"No patch files found in {source_dir}"
        )]

    results = []
    alerts = []

    for source_patch in source_patches:
        target_patch = target_path / source_patch.name
        patch_name = source_patch.name

        # Read source content
        with open(source_patch, 'r') as f:
            source_content = f.read()

        # Check if target exists and differs
        overwritten_diff = None
        if target_patch.exists():
            with open(target_patch, 'r') as f:
                target_content = f.read()
            if source_content != target_content:
                # Capture diff for reporting
                returncode, diff_output, _ = run_command([
                    "diff", "-u",
                    "--label", f"source ({source_dir}{patch_name})",
                    "--label", f"target ({target_dir}{patch_name})",
                    str(source_patch), str(target_patch)
                ])
                overwritten_diff = diff_output

        if dry_run:
            if target_patch.exists():
                if overwritten_diff:
                    results.append(f"  [WOULD OVERWRITE] {patch_name} (has differences)")
                else:
                    results.append(f"  [WOULD COPY] {patch_name} (identical)")
            else:
                results.append(f"  [WOULD CREATE] {patch_name}")
        else:
            shutil.copy2(str(source_patch), str(target_patch))
            if overwritten_diff:
                results.append(f"  COPIED {patch_name} (differences overwritten - see below)")
                alerts.append((patch_name, overwritten_diff))
            elif target_patch.exists():
                results.append(f"  COPIED {patch_name} (was identical)")
            else:
                results.append(f"  CREATED {patch_name}")

    # Check for stale patches in target that don't exist in source
    source_patch_names = {p.name for p in source_patches}
    target_patches = {p.name for p in target_path.glob("*.patch")}
    stale_patches = target_patches - source_patch_names

    # Build output
    mode = "Dry Run" if dry_run else "Results"
    output_lines = [f"## Patch Sync {mode}\n"]
    output_lines.append(f"**Source**: {source_dir}")
    output_lines.append(f"**Target**: {target_dir}")
    output_lines.append(f"**Patches**: {len(source_patches)}\n")
    output_lines.extend(results)

    if stale_patches:
        output_lines.append(f"\n### Stale Patches in Target")
        output_lines.append("These patches exist in the target but not in the source:")
        for name in sorted(stale_patches):
            output_lines.append(f"  - {name} (consider removing)")

    if alerts:
        output_lines.append("\n### Overwritten Differences\n")
        output_lines.append("The following patches had differences that were overwritten.")
        output_lines.append("These differences are expected to be safe to discard (historically")
        output_lines.append("FIPS patches had `+++ b/authentik-fips/` path prefixes, but this was")
        output_lines.append("a bug since melange applies patches against the unchanged `authentik/` directory).\n")

        for patch_name, diff in alerts:
            output_lines.append(f"#### {patch_name}")
            output_lines.append("```diff")
            diff_lines = diff.split('\n')
            if len(diff_lines) > 80:
                output_lines.extend(diff_lines[:80])
                output_lines.append(f"... ({len(diff_lines) - 80} more lines)")
            else:
                output_lines.extend(diff_lines)
            output_lines.append("```\n")

    return [TextContent(type="text", text="\n".join(output_lines))]


async def update_package_yaml(
    package_name: str,
    version: str,
    commit: str,
    patches: Optional[List[str]] = None
) -> list[TextContent]:
    """Update package YAML file with new version and commit."""

    # Determine file path
    yaml_file = Path(f"enterprise-packages/{package_name}.yaml")

    if not yaml_file.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: Package file not found: {yaml_file}"
        )]

    try:
        if has_ruamel:
            # Use ruamel.yaml to preserve formatting and comments
            ryaml = YAML()
            ryaml.preserve_quotes = True
            ryaml.width = 4096

            with open(yaml_file, 'r') as f:
                data = ryaml.load(f)

            # Update version
            data['package']['version'] = version

            # Find and update expected-commit in pipeline
            for step in data.get('pipeline', []):
                if step.get('uses') == 'git-checkout':
                    if 'with' in step:
                        step['with']['expected-commit'] = commit

            # Update patches if provided
            if patches is not None:
                for step in data.get('pipeline', []):
                    if step.get('uses') == 'patch':
                        if 'with' in step:
                            step['with']['patches'] = "\n".join(patches) + "\n"

            # Write back
            with open(yaml_file, 'w') as f:
                ryaml.dump(data, f)

        else:
            # Fallback to pyyaml (will lose formatting)
            with open(yaml_file, 'r') as f:
                data = yaml.safe_load(f)

            data['package']['version'] = version

            for step in data.get('pipeline', []):
                if step.get('uses') == 'git-checkout':
                    if 'with' in step:
                        step['with']['expected-commit'] = commit

            if patches is not None:
                for step in data.get('pipeline', []):
                    if step.get('uses') == 'patch':
                        if 'with' in step:
                            step['with']['patches'] = "\n".join(patches) + "\n"

            with open(yaml_file, 'w') as f:
                yaml.dump(data, f, default_flow_style=False, sort_keys=False)

        return [TextContent(
            type="text",
            text=f"Successfully updated {yaml_file}\n\nUpdated:\n  - version: {version}\n  - commit: {commit}" +
                 (f"\n  - patches: {len(patches)} files" if patches else "")
        )]

    except Exception as e:
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to update {yaml_file}: {str(e)}"
        )]


async def cleanup(temp_dir: str) -> list[TextContent]:
    """Remove a temporary directory."""

    if not temp_dir:
        return [TextContent(type="text", text="ERROR: temp_dir parameter is required")]

    temp_path = Path(temp_dir)

    if not temp_path.exists():
        return [TextContent(
            type="text",
            text=f"Warning: Directory does not exist: {temp_dir}"
        )]

    try:
        shutil.rmtree(temp_dir)

        # Remove from tracked repos
        for repo_key, path in list(_temp_repos.items()):
            if path == temp_dir or str(temp_path) in path:
                del _temp_repos[repo_key]

        # Clear analysis cache
        _analysis_cache.clear()

        return [TextContent(
            type="text",
            text=f"Cleaned up temporary directory: {temp_dir}"
        )]
    except Exception as e:
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to remove directory {temp_dir}: {str(e)}"
        )]


if __name__ == "__main__":
    import asyncio
    from mcp.server.stdio import stdio_server

    async def main():
        async with stdio_server() as (read_stream, write_stream):
            await app.run(
                read_stream,
                write_stream,
                app.create_initialization_options()
            )

    asyncio.run(main())
