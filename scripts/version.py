#!/usr/bin/env python3
"""
Version management script for browserfleet
This script helps with manual version management when needed
"""
import os
import re
import subprocess
import sys
import argparse
import toml


def get_current_version():
    """Read current version from pyproject.toml"""
    with open("pyproject.toml", "r") as f:
        config = toml.load(f)
    return config["project"]["version"]


def update_version_in_pyproject(new_version):
    """Update version in pyproject.toml"""
    with open("pyproject.toml", "r") as f:
        content = f.read()
    
    pattern = r'version = "[0-9]+\.[0-9]+\.[0-9]+"'
    replacement = f'version = "{new_version}"'
    new_content = re.sub(pattern, replacement, content)
    
    with open("pyproject.toml", "w") as f:
        f.write(new_content)


def create_git_tag(version):
    """Create an annotated git tag for the version"""
    tag_name = f"v{version}"
    subprocess.run(["git", "tag", "-a", tag_name, "-m", f"Release {tag_name}"], check=True)
    print(f"Created git tag: {tag_name}")
    print("To push the tag, run: git push origin " + tag_name)


def extract_versions(version_str):
    """Extract major, minor, and patch from the 0.X.Y format
    Handles special encoding of major version in minor position"""
    _, encoded_minor, patch = map(int, version_str.split("."))
    
    # Extract major and minor from encoded minor position
    # If minor is >= 100, it encodes a major version
    if encoded_minor >= 100:
        major = encoded_minor // 100
        minor = encoded_minor % 100
    else:
        major = 0
        minor = encoded_minor
        
    return major, minor, patch


def bump_version(current_version, bump_type):
    """Bump version according to semantic versioning with major encoded in minor"""
    major, minor, patch = extract_versions(current_version)
    
    if bump_type == "major":
        major += 1
        minor = 1  # Reset minor to 1 when bumping major
        patch = 0  # Reset patch
        return f"0.{(major * 100) + minor}.{patch}"
    elif bump_type == "minor":
        minor += 1
        patch = 0  # Reset patch
        return f"0.{(major * 100) + minor}.{patch}"
    elif bump_type == "patch":
        patch += 1
        return f"0.{(major * 100) + minor}.{patch}"
    else:
        raise ValueError(f"Invalid bump type: {bump_type}")


def main():
    parser = argparse.ArgumentParser(description="Version management tool")
    parser.add_argument("action", choices=["bump", "set", "get"], help="Action to perform")
    parser.add_argument("--type", choices=["major", "minor", "patch"], help="Version bump type")
    parser.add_argument("--version", help="Set specific version (for 'set' action)")
    parser.add_argument("--tag", action="store_true", help="Create git tag")
    
    args = parser.parse_args()
    
    try:
        current_version = get_current_version()
        
        if args.action == "get":
            major, minor, patch = extract_versions(current_version)
            print(f"Current version: {current_version} (Major: {major}, Minor: {minor}, Patch: {patch})")
            return
        
        if args.action == "bump":
            if not args.type:
                parser.error("--type is required for bump action")
            new_version = bump_version(current_version, args.type)
        elif args.action == "set":
            if not args.version:
                parser.error("--version is required for set action")
            # Validate version format
            if not re.match(r"^0\.\d+\.\d+$", args.version):
                parser.error(f"Invalid version format: {args.version}. Expected format: 0.x.y")
            new_version = args.version
        
        print(f"Updating version: {current_version} -> {new_version}")
        update_version_in_pyproject(new_version)
        
        if args.tag:
            create_git_tag(new_version)
            
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main() 