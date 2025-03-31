# browsergrid
easy browser infrastructure with captcha solving

## Versioning Strategy

This project follows a modified semantic versioning (SemVer) strategy with a 0.x.y format:

- **Major version** is fixed at 0 during initial development
- **Minor version (x)** follows a special encoding:
  - Regular minor bumps: 1-99
  - Major version bumps: encoded as 100*major + minor
  - Example: 0.102.0 represents major version 1, minor version 2
- **Patch version (y)** increments for bug fixes and non-breaking improvements

### How We Version

- Version numbers are managed through git tags (e.g., `v0.1.0`)
- All releases use annotated tags for better documentation
- Version bumping is automated through GitHub Actions
- Contributors must label PRs appropriately as "major", "minor", or "patch"

### Examples

- 0.1.0 - Initial version
- 0.2.0 - Minor version bump
- 0.102.0 - Major version bump (represents major:1, minor:2)
- 0.102.1 - Patch to version 1.2.0

### Contributing

When submitting a pull request:
1. Apply the appropriate label: "major", "minor", or "patch"
2. Follow the PR template guidelines
3. Maintain a clean commit history
