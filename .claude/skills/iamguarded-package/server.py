#!/usr/bin/env python3
"""
IAMGuarded Package MCP Server

Provides tools for automating IAMGuarded compat package creation.
"""

import os
import subprocess
import tempfile
import shutil
import json
from pathlib import Path
from typing import Optional, Dict, Any, List
import yaml
import re

from mcp.server import Server
from mcp.types import Tool, TextContent

# Global state for managing temporary directories
_temp_repos: Dict[str, str] = {}

app = Server("iamguarded-package")


def validate_working_directory() -> Optional[str]:
    """Validate that we're in the enterprise-packages repository root."""
    if not Path("enterprise-packages").exists() and not Path(".git").exists():
        return "ERROR: Must be run from enterprise-packages repository root"

    # Check if this is actually a git repo
    returncode, stdout, stderr = run_command([
        "git", "rev-parse", "--git-dir"
    ])
    if returncode != 0:
        return [TextContent(
            type="text",
            text=f"ERROR: {stderr}"
        )]
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
            name="checkout_repo",
            description="Clone a repository (iamguarded-tools, iamguarded-containers, or iamguarded-charts) to a temporary directory. Returns the path to the cloned repo. Maintains state to avoid re-cloning.",
            inputSchema={
                "type": "object",
                "properties": {
                    "repo_name": {
                        "type": "string",
                        "description": "Repository name: 'iamguarded-tools', 'iamguarded-containers', or 'iamguarded-charts'",
                        "enum": ["iamguarded-tools", "iamguarded-containers", "iamguarded-charts"]
                    }
                },
                "required": ["repo_name"]
            }
        ),
        Tool(
            name="cleanup",
            description="Remove a temporary directory created by checkout_repo.",
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
        Tool(
            name="find_package",
            description="Find the package YAML file for a given image name in enterprise-packages.",
            inputSchema={
                "type": "object",
                "properties": {
                    "image_name": {
                        "type": "string",
                        "description": "Image/package name (e.g., 'mysql', 'zookeeper')"
                    }
                },
                "required": ["image_name"]
            }
        ),
        Tool(
            name="has_bitnami_compat",
            description="Check if a package has a bitnami-compat subpackage. Returns 'true' or 'false'.",
            inputSchema={
                "type": "object",
                "properties": {
                    "image_name": {
                        "type": "string",
                        "description": "Image/package name (e.g., 'mysql', 'zookeeper')"
                    }
                },
                "required": ["image_name"]
            }
        ),
        Tool(
            name="get_version_variable",
            description="Determine the version variable type (major-version or major-minor-version) from a package's version-path.",
            inputSchema={
                "type": "object",
                "properties": {
                    "image_name": {
                        "type": "string",
                        "description": "Image/package name (e.g., 'mysql', 'zookeeper')"
                    }
                },
                "required": ["image_name"]
            }
        ),
        Tool(
            name="show_package_diff",
            description="Show the git diff for a package file.",
            inputSchema={
                "type": "object",
                "properties": {
                    "package_file": {
                        "type": "string",
                        "description": "Path to the package file (e.g., 'mysql.yaml')"
                    }
                },
                "required": ["package_file"]
            }
        ),
        Tool(
            name="analyze_image_paths",
            description="Analyze a container image using dive to extract file/directory paths and permissions. Filters results to only include paths from: /opt, /usr/bin, /etc/, /usr/local/, /bin, /sbin, /lib, /usr/lib, /lib64, and /var/run (including subdirectories).",
            inputSchema={
                "type": "object",
                "properties": {
                    "image": {
                        "type": "string",
                        "description": "Container image reference (e.g., 'bitnamilegacy/zookeeper:latest')"
                    }
                },
                "required": ["image"]
            }
        ),
    ]


@app.call_tool()
async def call_tool(name: str, arguments: Any) -> list[TextContent]:
    """Handle tool calls."""

    # Validate working directory for all tools
    error = validate_working_directory()
    if error:
        return [TextContent(type="text", text=error)]

    if name == "checkout_repo":
        return await checkout_repo(arguments["repo_name"])
    elif name == "cleanup":
        return await cleanup(arguments["temp_dir"])
    elif name == "find_package":
        return await find_package(arguments["image_name"])
    elif name == "has_bitnami_compat":
        return await has_bitnami_compat(arguments["image_name"])
    elif name == "get_version_variable":
        return await get_version_variable(arguments["image_name"])
    elif name == "show_package_diff":
        return await show_package_diff(arguments["package_file"])
    elif name == "analyze_image_paths":
        return await analyze_image_paths(arguments["image"])
    else:
        return [TextContent(type="text", text=f"ERROR: Unknown tool: {name}")]


async def checkout_repo(repo_name: str) -> list[TextContent]:
    """Clone a repository to a temporary directory."""

    # Check if already cloned
    if repo_name in _temp_repos:
        repo_path = _temp_repos[repo_name]
        if Path(repo_path).exists():
            return [TextContent(
                type="text",
                text=f"Repository already cloned at: {repo_path}"
            )]

    # Create temp directory
    temp_base = tempfile.mkdtemp(prefix="iamguarded-")
    repo_path = Path(temp_base) / repo_name

    # Clone the repository
    repo_url = f"https://github.com/chainguard-dev/{repo_name}.git"
    returncode, stdout, stderr = run_command([
        "git", "clone", "-q", "--depth", "50", repo_url, str(repo_path)
    ])

    if returncode != 0:
        shutil.rmtree(temp_base, ignore_errors=True)
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to clone {repo_name} repository\n{stderr}"
        )]

    # Store the path
    _temp_repos[repo_name] = str(repo_path)

    return [TextContent(
        type="text",
        text=str(repo_path)
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
        for repo_name, path in list(_temp_repos.items()):
            if path == temp_dir or str(temp_path) in path:
                del _temp_repos[repo_name]

        return [TextContent(
            type="text",
            text=f"Cleaned up temporary directory: {temp_dir}"
        )]
    except Exception as e:
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to remove directory {temp_dir}: {str(e)}"
        )]


async def find_package(image_name: str) -> list[TextContent]:
    """Find the package YAML file for an image."""

    if not image_name:
        return [TextContent(type="text", text="ERROR: image_name parameter is required")]

    package_file = Path(f"{image_name}.yaml")

    if not package_file.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: Package file not found: {package_file}"
        )]

    return [TextContent(type="text", text=str(package_file))]


async def has_bitnami_compat(image_name: str) -> list[TextContent]:
    """Check if a package has a bitnami-compat subpackage."""

    if not image_name:
        return [TextContent(type="text", text="ERROR: image_name parameter is required")]

    package_file = Path(f"{image_name}.yaml")

    if not package_file.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: Package file not found: {package_file}"
        )]

    try:
        with open(package_file, 'r') as f:
            data = yaml.safe_load(f)

        # Check subpackages for bitnami-compat
        subpackages = data.get('subpackages', [])
        for subpkg in subpackages:
            name = subpkg.get('name', '')
            if f"{image_name}-bitnami-compat" in name:
                return [TextContent(type="text", text="true")]

        return [TextContent(type="text", text="false")]

    except Exception as e:
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to parse {package_file}: {str(e)}"
        )]


async def get_version_variable(image_name: str) -> list[TextContent]:
    """Determine the version variable type from version-path."""

    if not image_name:
        return [TextContent(type="text", text="ERROR: image_name parameter is required")]

    package_file = Path(f"{image_name}.yaml")

    if not package_file.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: Package file not found: {package_file}"
        )]

    try:
        with open(package_file, 'r') as f:
            data = yaml.safe_load(f)

        # Get version-path from package metadata
        version_path = data.get('package', {}).get('version-path')

        if not version_path or version_path == 'null':
            # No version-path, default to major-version
            return [TextContent(type="text", text="major-version")]

        # If version-path contains a dot (e.g., "1.29/debian-12"), use major-minor
        if re.match(r'^\d+\.\d+', str(version_path)):
            return [TextContent(type="text", text="major-minor-version")]
        else:
            return [TextContent(type="text", text="major-version")]

    except Exception as e:
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to parse {package_file}: {str(e)}"
        )]


async def show_package_diff(package_file: str) -> list[TextContent]:
    """Show the git diff for a package file."""

    if not package_file:
        return [TextContent(type="text", text="ERROR: package_file parameter is required")]

    pkg_path = Path(package_file)

    if not pkg_path.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: File does not exist: {package_file}"
        )]

    returncode, stdout, stderr = run_command(["git", "diff", package_file])

    if returncode != 0:
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to get diff for {package_file}\n{stderr}"
        )]

    if not stdout:
        return [TextContent(
            type="text",
            text=f"No changes to {package_file}"
        )]

    return [TextContent(
        type="text",
        text=f"Changes to {package_file}:\n\n{stdout}"
    )]


async def analyze_image_paths(image: str) -> list[TextContent]:
    """Analyze container image paths and permissions using dive."""

    if not image:
        return [TextContent(type="text", text="ERROR: image parameter is required")]

    # Define allowed path prefixes
    allowed_prefixes = (
        'opt',
        'bitnami'
    )

    # Create temporary file for dive output
    with tempfile.NamedTemporaryFile(mode='w+', suffix='.json', delete=False) as tmp_file:
        tmp_path = tmp_file.name

    try:
        # Run dive with JSON output
        returncode, stdout, stderr = run_command([
            "dive", image, "-j", tmp_path
        ])

        if returncode != 0:
            return [TextContent(
                type="text",
                text=f"ERROR: Failed to analyze image with dive\n{stderr}"
            )]

        # Read and parse the JSON output
        with open(tmp_path, 'r') as f:
            dive_data = json.load(f)

        # Filter the data to only include relevant paths
        filtered_data = filter_dive_output(dive_data, allowed_prefixes)

        # Return the filtered data as JSON
        return [TextContent(
            type="text",
            text=json.dumps(filtered_data, indent=2)
        )]

    except json.JSONDecodeError as e:
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to parse dive JSON output: {str(e)}"
        )]
    except Exception as e:
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to analyze image: {str(e)}"
        )]
    finally:
        # Clean up temporary file
        try:
            os.unlink(tmp_path)
        except:
            pass


def filter_dive_output(dive_data: Dict[str, Any], allowed_prefixes: tuple) -> Dict[str, Any]:
    """
    Filter dive JSON output to only include paths from allowed directories.

    Removes entire objects that don't have paths matching the allowed prefixes.
    """
    filtered = {
        "image": dive_data.get("image", {}),
        "layers": []
    }

    # Process each layer
    for layer in dive_data.get("layer", []):
        filtered_layer = {
            "index": layer.get("index"),
            "id": layer.get("id"),
            "digestId": layer.get("digestId"),
            "sizeBytes": layer.get("sizeBytes"),
            "command": layer.get("command"),
            "fileList": []
        }

        # Filter fileList to only include allowed paths
        for item in layer.get("fileList", []):
            path = item.get("path", "")

            # Check if path starts with any allowed prefix
            if any(path.startswith(prefix) for prefix in allowed_prefixes):
                filtered_item = {
                    "path": item.get("path"),
                    "linkName": item.get("linkName"),
                    "isDir": item.get("isDir")
                }
                filtered_layer["fileList"].append(filtered_item)

        # Only add layer if it has fileList entries
        if filtered_layer["fileList"]:
            filtered["layers"].append(filtered_layer)

    return filtered


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
