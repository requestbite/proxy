#!/usr/bin/env bash
#
# release.sh - Create GitHub release with cross-platform binaries
#
# Usage: ./scripts/release.sh v0.3.0
#

set -euo pipefail

# Colors for output
COLOR_RESET='\033[0m'
COLOR_BOLD='\033[1m'
COLOR_GREEN='\033[32m'
COLOR_BLUE='\033[34m'
COLOR_RED='\033[31m'
COLOR_YELLOW='\033[33m'

# Utility functions
info() {
    echo -e "${COLOR_BOLD}${COLOR_BLUE}==>${COLOR_RESET} ${COLOR_BOLD}$*${COLOR_RESET}"
}

success() {
    echo -e "${COLOR_GREEN}✓${COLOR_RESET} $*"
}

error() {
    echo -e "${COLOR_RED}✗ Error:${COLOR_RESET} $*" >&2
}

warning() {
    echo -e "${COLOR_YELLOW}⚠ Warning:${COLOR_RESET} $*"
}

die() {
    error "$*"
    exit 1
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Validate prerequisites
check_prerequisites() {
    info "Checking prerequisites..."

    local missing=()

    if ! command_exists gh; then
        missing+=("gh (GitHub CLI)")
    fi

    if ! command_exists git; then
        missing+=("git")
    fi

    if ! command_exists go; then
        missing+=("go")
    fi

    if ! command_exists make; then
        missing+=("make")
    fi

    if [ ${#missing[@]} -gt 0 ]; then
        error "Missing required tools:"
        for tool in "${missing[@]}"; do
            echo "  - $tool"
        done
        echo ""
        echo "Install instructions:"
        echo "  gh:   https://cli.github.com/"
        echo "  git:  https://git-scm.com/"
        echo "  go:   https://golang.org/doc/install"
        echo "  make: Usually pre-installed on Unix systems"
        exit 1
    fi

    # Check gh authentication
    if ! gh auth status >/dev/null 2>&1; then
        die "GitHub CLI not authenticated. Run: gh auth login"
    fi

    success "All prerequisites satisfied"
}

# Validate version tag format
validate_version_tag() {
    local tag="$1"

    if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        die "Invalid version tag format: $tag (expected: v*.*.*, e.g., v0.3.0)"
    fi
}

# Extract version from tag (remove 'v' prefix)
extract_version() {
    local tag="$1"
    echo "${tag#v}"
}

# Verify version in main.go matches tag
verify_version_match() {
    local tag="$1"
    local version
    version=$(extract_version "$tag")

    info "Verifying version in main.go matches tag..."

    local code_version
    code_version=$(grep 'Version.*=' main.go | sed 's/.*"\(.*\)"/\1/')

    if [ "$code_version" != "$version" ]; then
        die "Version mismatch: tag is $version but main.go has $code_version"
    fi

    success "Version matches: $version"
}

# Check for uncommitted changes
check_clean_working_dir() {
    info "Checking for uncommitted changes..."

    if ! git diff-index --quiet HEAD -- 2>/dev/null; then
        warning "Uncommitted changes detected"
        echo ""
        git status --short
        echo ""
        read -p "Continue anyway? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            die "Aborted by user"
        fi
    else
        success "Working directory is clean"
    fi
}

# Check if tag already exists
check_tag_exists() {
    local tag="$1"

    if git rev-parse "$tag" >/dev/null 2>&1; then
        warning "Tag $tag already exists locally"
        read -p "Continue? This will use the existing tag. (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            die "Aborted by user"
        fi
    fi
}

# Build release artifacts
build_release() {
    info "Building release artifacts..."

    if ! make release; then
        die "Build failed"
    fi

    success "Build complete"
}

# Verify release artifacts exist
verify_artifacts() {
    info "Verifying release artifacts..."

    local dist_dir="dist"
    local required_files=(
        "$dist_dir/SHA256SUMS"
    )

    # Check for at least one archive
    local archive_count
    archive_count=$(find "$dist_dir" -name "*.tar.gz" -o -name "*.zip" | wc -l)

    if [ "$archive_count" -eq 0 ]; then
        die "No release archives found in $dist_dir/"
    fi

    # Check SHA256SUMS exists
    if [ ! -f "$dist_dir/SHA256SUMS" ]; then
        die "SHA256SUMS file not found"
    fi

    success "Found $archive_count release archive(s)"
}

# Get repository info
get_repo_info() {
    local remote_url
    remote_url=$(git remote get-url origin 2>/dev/null || echo "")

    if [ -z "$remote_url" ]; then
        die "No git remote 'origin' configured"
    fi

    # Extract owner/repo from various URL formats
    if [[ "$remote_url" =~ github.com[:/]([^/]+)/([^/.]+)(\.git)?$ ]]; then
        echo "${BASH_REMATCH[1]}/${BASH_REMATCH[2]}"
    else
        die "Could not parse repository from remote URL: $remote_url"
    fi
}

# Generate release notes
generate_release_notes() {
    local tag="$1"
    local version
    version=$(extract_version "$tag")

    cat <<EOF
# RequestBite Proxy v${version}

Cross-platform HTTP forwarding proxy for bypassing CORS restrictions.

## Installation

### Quick Install (macOS/Linux)

\`\`\`bash
curl -fsSL https://raw.githubusercontent.com/requestbite/proxy-go/main/install.sh | bash
\`\`\`

### Manual Download

Download the appropriate archive for your platform below, extract it, and move the binary to your PATH.

## Platform Support

- **macOS Intel (x86-64)**: \`requestbite-proxy-${version}-darwin-amd64.tar.gz\`
- **macOS Apple Silicon (ARM64)**: \`requestbite-proxy-${version}-darwin-arm64.tar.gz\`
- **Linux x86-64**: \`requestbite-proxy-${version}-linux-amd64.tar.gz\`
- **Windows x86-64**: \`requestbite-proxy-${version}-windows-amd64.zip\`

## Checksums

SHA256 checksums are provided in \`SHA256SUMS\`. Verify your download:

\`\`\`bash
shasum -a 256 -c SHA256SUMS
\`\`\`

## Usage

\`\`\`bash
requestbite-proxy --port 8080
\`\`\`

For more information, see the [README](https://github.com/requestbite/proxy-go#readme).

---

*Built with [Go](https://golang.org/)*
EOF
}

# Create GitHub release
create_github_release() {
    local tag="$1"
    local repo
    repo=$(get_repo_info)

    info "Creating GitHub release for $tag..."

    # Generate release notes
    local notes_file
    notes_file=$(mktemp)
    generate_release_notes "$tag" > "$notes_file"

    # Create release
    if gh release create "$tag" \
        --repo "$repo" \
        --title "Release $tag" \
        --notes-file "$notes_file" \
        dist/*.tar.gz \
        dist/*.zip \
        dist/SHA256SUMS; then
        rm -f "$notes_file"
        success "Release created successfully"
        echo ""
        echo "View release: https://github.com/$repo/releases/tag/$tag"
        return 0
    else
        rm -f "$notes_file"
        die "Failed to create release"
    fi
}

# Main function
main() {
    if [ $# -ne 1 ]; then
        echo "Usage: $0 <version-tag>"
        echo ""
        echo "Example: $0 v0.3.0"
        echo ""
        echo "This script will:"
        echo "  1. Validate the version tag format"
        echo "  2. Verify the version matches main.go"
        echo "  3. Check for uncommitted changes"
        echo "  4. Build release artifacts for all platforms"
        echo "  5. Create a GitHub release with all artifacts"
        exit 1
    fi

    local tag="$1"

    echo ""
    info "RequestBite Proxy - Release Script"
    echo ""

    # Validate and prepare
    validate_version_tag "$tag"
    check_prerequisites
    check_clean_working_dir
    verify_version_match "$tag"
    check_tag_exists "$tag"

    echo ""

    # Build
    build_release
    verify_artifacts

    echo ""

    # Create release
    create_github_release "$tag"

    echo ""
    success "Release process complete!"
    echo ""
    echo "Next steps:"
    echo "  1. Verify the release on GitHub"
    echo "  2. Test the installation script"
    echo "  3. Announce the release"
}

# Run main function
main "$@"
