# Versioning Strategy

## Overview

Browserfleet follows a modified semantic versioning (SemVer) approach with a 0.x.y format:

- **Major version**: Fixed at 0 during initial development
- **Minor version (x)**: 
  - Regular minor bumps: 1-99
  - Major bumps: encoded as 100*n + minor (e.g., 102 = major 1, minor 2)
- **Patch version (y)**: Increments for bug fixes and non-breaking improvements

This approach helps maintain a clear development history while acknowledging the pre-1.0 status of the project.

## Version Numbering Rules

1. **Starting Version**: 0.1.0
2. **Minor Version Increments**: For new features or significant improvements (non-breaking)
   - Example: 0.1.0 → 0.2.0
3. **Major Version Encoding**: For breaking changes, encoded in the minor position
   - Formula: 0.(100*major + minor).0
   - Example: 0.2.0 → 0.102.0 (represents major version 1, minor version 2)
   - Example: 0.199.0 → 0.201.0 (from major 1, minor 99 to major 2, minor 1)
4. **Patch Version Increments**: For bug fixes and non-breaking improvements
   - Example: 0.1.0 → 0.1.1
5. **Reset Patch Version**: When bumping minor or major version, reset patch version to 0
   - Example: 0.1.2 → 0.2.0 or 0.102.0

## Git Tagging

All releases are tagged in Git using annotated tags:

```bash
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

## Automated Versioning

We use GitHub Actions to automate version bumping:

1. PRs must be labeled as either "minor" or "patch"
2. Upon merge to main, the version is automatically bumped
3. A new git tag is created and pushed
4. The version in `pyproject.toml` is updated

## Manual Versioning

For manual version management, use the provided script:

```bash
# Get current version
python scripts/version.py get

# Bump minor version
python scripts/version.py bump --type minor --tag

# Bump major version (encoded in minor position)
python scripts/version.py bump --type major --tag

# Bump patch version
python scripts/version.py bump --type patch --tag

# Set specific version
python scripts/version.py set --version 0.2.1 --tag
```

## Branch Protection

To ensure proper versioning:

1. The `main` branch is protected
2. PRs require at least one approval
3. PRs must pass the validation check (has proper version label)
4. Signed commits are required
5. Linear history is maintained

## For Contributors

When contributing to this project:

1. Create a feature branch from `main`
2. Make your changes with descriptive commit messages
3. Open a PR with the appropriate label ("minor" or "patch")
4. Ensure all checks pass
5. Get approval from maintainers
6. Your change will be merged and versioned automatically 