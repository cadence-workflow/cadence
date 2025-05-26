#!/bin/bash

set -e

# Ensure we're running in bash
if [ -z "$BASH_VERSION" ]; then
    echo "This script requires bash. Please run with: bash $0 $@"
    exit 1
fi

# Portable function to get relative path
get_relative_path() {
    local target="$1"
    local base="$2"

    # Convert to absolute paths without cd (in case dirs don't exist)
    if [[ "$target" = /* ]]; then
        # Already absolute
        target="$target"
    else
        target="$(pwd)/$target"
    fi

    if [[ "$base" = /* ]]; then
        # Already absolute
        base="$base"
    else
        base="$(pwd)/$base"
    fi

    # If same directory, return "."
    if [[ "$target" == "$base" ]]; then
        echo "."
        return
    fi

    # Calculate relative path
    local result="${target#$base/}"

    # If no match, return the target path relative to base
    if [[ "$result" == "$target" ]]; then
        echo "$target"
    else
        echo "$result"
    fi
}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git rev-parse --show-toplevel)"
DRY_RUN=false
PRERELEASE=false
VERSION=""
VERSION_TYPE=""

# Usage function
usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Create releases for all Go modules in the repository with unified versioning.
All modules will receive the same version number.

OPTIONS:
    -t, --type TYPE     Version bump type: major, minor, patch
    -v, --version VER   Specific version to set for all modules (overrides --type)
    -p, --prerelease    Create prerelease versions (adds -prereleaseNN suffix)
                        Can be used alone to increment prerelease number only
    -d, --dry-run       Show what would be done without making changes
    -h, --help          Show this help message

EXAMPLES:
    $0 --type patch                    # Bump patch version for all modules
    $0 --type minor --prerelease       # Bump minor version with prerelease suffix
    $0 --version v1.2.3               # Set v1.2.3 for all modules
    $0 --version v1.2.3 --prerelease  # Set v1.2.3-prerelease01 for all modules
    $0 --prerelease                   # Just increment prerelease number (no version bump)
    $0 --type major --dry-run          # Show what would happen with major bump

PRERELEASE WORKFLOWS:
    # Start prerelease cycle for v1.2.4
    $0 --type patch --prerelease       # Creates v1.2.4-prerelease01

    # Continue prerelease iterations (no version bump)
    $0 --prerelease                   # Creates v1.2.4-prerelease02
    $0 --prerelease                   # Creates v1.2.4-prerelease03

    # Final release
    $0 --version v1.2.4               # Creates v1.2.4 (removes prerelease suffix)

UNIFIED VERSIONING:
    All modules will receive the same version based on the highest current version
    across all modules. For example, if the current highest version is v1.2.3,
    running with --type patch will create v1.2.4 for ALL modules.

PRERELEASE FORMAT:
    Prereleases use STRICT 2-digit numbering (01-99) for proper alphabetical sorting:
    v1.2.3-prerelease01, v1.2.3-prerelease02, ..., v1.2.3-prerelease99

    LEGACY SUPPORT: Historical 1-digit prerelease tags are ignored unless they are
    the latest prerelease for a specific version. Only the latest prerelease matters
    for determining the next prerelease number.

SAFETY FEATURES:
    - Prevents creating versions that already exist for any module
    - Verifies builds and runs tests before creating tags
    - Requires clean git working directory
    - Enforces releases only from master branch

EOF
}

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -t|--type)
                VERSION_TYPE="$2"
                shift 2
                ;;
            -v|--version)
                VERSION="$2"
                shift 2
                ;;
            -p|--prerelease)
                PRERELEASE=true
                shift
                ;;
            -d|--dry-run)
                DRY_RUN=true
                shift
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done

    # Validate arguments
    if [[ -z "$VERSION" && -z "$VERSION_TYPE" && "$PRERELEASE" != true ]]; then
        log_error "Either --version, --type, or --prerelease (alone) must be specified"
        usage
        exit 1
    fi

    if [[ -n "$VERSION" && -n "$VERSION_TYPE" ]]; then
        log_error "Cannot specify both --version and --type"
        usage
        exit 1
    fi

    if [[ -n "$VERSION_TYPE" && ! "$VERSION_TYPE" =~ ^(major|minor|patch)$ ]]; then
        log_error "Version type must be one of: major, minor, patch"
        exit 1
    fi

    # If only --prerelease is specified, we'll increment prerelease number only
    if [[ -z "$VERSION" && -z "$VERSION_TYPE" && "$PRERELEASE" == true ]]; then
        VERSION_TYPE="prerelease-only"
    fi
}

# Find all go.mod files, excluding specified directories
find_go_modules() {
    local modules=()
    local seen_modules=()

    # Find all go.mod files
    while IFS= read -r -d '' file; do
        local dir=$(dirname "$file")
        local rel_dir=$(get_relative_path "$dir" "$REPO_ROOT")

        # Normalize relative directory (remove ./ prefix)
        if [[ "$rel_dir" == "./" ]]; then
            rel_dir="."
        fi
        rel_dir="${rel_dir#./}"

        # Skip if in cmd directory or subdirectories
        if [[ "$rel_dir" =~ ^cmd/ ]] || [[ "$rel_dir" == "cmd" ]]; then
            continue
        fi

        # Skip if in internal/tools directory
        if [[ "$rel_dir" == "internal/tools" ]]; then
            continue
        fi

        # Skip if in idls directory (submodule)
        if [[ "$rel_dir" == "idls" ]]; then
            continue
        fi

        # Check for duplicates
        local found_duplicate=false
        for seen in "${seen_modules[@]}"; do
            if [[ "$seen" == "$dir" ]]; then
                found_duplicate=true
                break
            fi
        done

        if [[ "$found_duplicate" == false ]]; then
            modules+=("$dir")
            seen_modules+=("$dir")
        fi
    done < <(find "$REPO_ROOT" -name "go.mod" -print0)

    printf '%s\n' "${modules[@]}"
}

# Get the current global version from all modules
get_current_global_version() {
    local all_tags=()

    # Collect all version tags from all modules (ignore prerelease format warnings)
    while IFS= read -r tag; do
        if [[ "$tag" =~ v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
            all_tags+=("$tag")
        fi
    done < <(git tag -l)

    if [[ ${#all_tags[@]} -eq 0 ]]; then
        echo "v0.0.0"
        return
    fi

    # Sort all tags and get the highest version
    local highest_tag=$(printf '%s\n' "${all_tags[@]}" | sort -V | tail -n1)

    # Extract just the version part (remove module prefix if present)
    local version=$(echo "$highest_tag" | sed 's/.*\(v[0-9][^-]*\)/\1/')

    echo "$version"
}

# Increment version based on type
increment_version() {
    local current_version="$1"
    local bump_type="$2"

    # Remove 'v' prefix for processing
    local version=$(echo "$current_version" | sed 's/^v//')

    # Handle prerelease versions (both 1-digit and 2-digit for parsing existing versions)
    if [[ "$version" =~ -prerelease[0-9]+$ ]]; then
        version=$(echo "$version" | sed 's/-prerelease[0-9]*$//')
    fi

    # Split version into components
    IFS='.' read -ra VERSION_PARTS <<< "$version"
    local major=${VERSION_PARTS[0]:-0}
    local minor=${VERSION_PARTS[1]:-0}
    local patch=${VERSION_PARTS[2]:-0}

    case "$bump_type" in
        major)
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        minor)
            minor=$((minor + 1))
            patch=0
            ;;
        patch)
            patch=$((patch + 1))
            ;;
    esac

    echo "${major}.${minor}.${patch}"
}

# Get next prerelease version globally (strict 2-digit format)
get_next_prerelease() {
    local base_version="$1"

    # Find existing prerelease tags for this specific version across all modules
    local all_prerelease_tags=$(git tag -l "*v${base_version}-prerelease*" | sort -V)

    if [[ -z "$all_prerelease_tags" ]]; then
        echo "${base_version}-prerelease01"
        return
    fi

    # Get the latest prerelease tag for this version
    local latest_prerelease=$(echo "$all_prerelease_tags" | tail -n1)

    # Check if the LATEST prerelease is 1-digit (this is the only one we care about)
    if [[ "$latest_prerelease" =~ -prerelease[0-9]$ ]] && [[ ! "$latest_prerelease" =~ -prerelease[0-9][0-9]$ ]]; then
        log_error "Latest prerelease tag for version ${base_version} uses 1-digit format: $latest_prerelease"
        log_error "Only 2-digit prerelease format is supported going forward (e.g., -prerelease01)."
        log_error "Please manually create the next prerelease with 2-digit format."
        return 1
    fi

    # Find only 2-digit prerelease tags for this version
    local two_digit_prereleases=$(git tag -l "*v${base_version}-prerelease[0-9][0-9]" | sort -V)

    if [[ -z "$two_digit_prereleases" ]]; then
        echo "${base_version}-prerelease01"
    else
        local latest_two_digit=$(echo "$two_digit_prereleases" | tail -n1)
        local prerelease_number=$(echo "$latest_two_digit" | sed 's/.*-prerelease//')
        local next_prerelease=$((prerelease_number + 1))

        # Ensure we don't exceed 99
        if [[ $next_prerelease -gt 99 ]]; then
            log_error "Maximum prerelease number (99) exceeded for version ${base_version}"
            log_error "Current highest: ${latest_two_digit}"
            return 1
        fi

        # Format with leading zero (2 digits)
        printf "${base_version}-prerelease%02d" "$next_prerelease"
    fi
}

# Check if version already exists for any module
check_version_exists() {
    local version="$1"
    local modules=("${@:2}")
    local existing_tags=()

    for module in "${modules[@]}"; do
        local module_name=""
        local expected_tag

        if [[ "$module" != "$REPO_ROOT" ]]; then
            module_name="$(get_relative_path "$module" "$REPO_ROOT")"
            expected_tag="${module_name}/v${version}"
        else
            expected_tag="v${version}"
        fi

        # Check if tag exists
        if git tag -l | grep -q "^${expected_tag}$"; then
            existing_tags+=("$expected_tag")
        fi
    done

    if [[ ${#existing_tags[@]} -gt 0 ]]; then
        log_error "Version v$version already exists for the following modules:"
        for tag in "${existing_tags[@]}"; do
            echo "  - $tag"
        done
        echo
        log_error "Cannot create release with existing version."
        return 1
    fi

    return 0
}

# Create release for a single module with given version
create_module_release() {
    local module_path="$1"
    local new_version="$2"
    local module_name=""

    cd "$module_path"

    # Construct full tag name
    local new_tag
    if [[ "$module_path" != "$REPO_ROOT" ]]; then
        module_name="$(get_relative_path "$module_path" "$REPO_ROOT")"
        new_tag="${module_name}/v${new_version}"
    else
        new_tag="v${new_version}"
    fi

    # Verify module builds
    if ! go mod tidy >/dev/null 2>&1; then
        log_error "Failed to tidy module: $new_tag"
        return 1
    fi

    log_info "Running go build to verify unbuildable code in module $module_name"
    if ! go build ./... >/dev/null 2>&1; then
        log_error "Failed to build module: $new_tag"
        return 1
    fi

    # Create and push tag
    cd "$REPO_ROOT"

    if git tag "$new_tag" >/dev/null 2>&1; then
        if git push origin "$new_tag" >/dev/null 2>&1; then
            echo "  ✓ $new_tag"
        else
            log_error "Failed to push tag: $new_tag"
            return 1
        fi
    else
        log_error "Failed to create tag: $new_tag"
        return 1
    fi
}

# Main function
main() {
    parse_args "$@"

    # Show mode
    if [[ -n "$VERSION" ]]; then
        echo "Mode: Set specific version (v$VERSION)"
    elif [[ "$VERSION_TYPE" == "prerelease-only" ]]; then
        echo "Mode: Increment prerelease number only"
    else
        echo "Mode: Version bump ($VERSION_TYPE)"
        if [[ "$PRERELEASE" == true ]]; then
            echo "      + prerelease suffix"
        fi
    fi

    if [[ "$DRY_RUN" == true ]]; then
        log_warning "DRY RUN MODE - No changes will be made"
    fi
    echo

    # Ensure we're in a clean git state (skip in dry-run mode)
    if [[ "$DRY_RUN" != true ]] && ! git diff-index --quiet HEAD --; then
        log_error "Working directory is not clean. Please commit or stash changes."
        log_error "Use --dry-run to see what would be done without requiring clean state."
        exit 1
    fig

    # Ensure we're on master branch
    local current_branch=$(git rev-parse --abbrev-ref HEAD)
    if [[ "$current_branch" != "master" ]]; then
        log_error "Must be on master branch to create releases. Current branch: $current_branch"
        log_error "Please switch to master branch: git checkout master"
        exit 1
    fi

    # Find all modules
    local modules=()
    while IFS= read -r module_path; do
        modules+=("$module_path")
    done < <(find_go_modules)

    if [[ ${#modules[@]} -eq 0 ]]; then
        log_error "No Go modules found"
        exit 1
    fi

    # Determine the current global version
    local current_version=$(get_current_global_version)

    # Calculate new version (same for all modules)
    local new_version
    if [[ -n "$VERSION" ]]; then
        # Strip 'v' prefix if present
        new_version=$(echo "$VERSION" | sed 's/^v//')

        # Handle prerelease for explicit version
        if [[ "$PRERELEASE" == true ]]; then
            # Check if the specified version already has prerelease suffix
            if [[ "$new_version" =~ -prerelease[0-9][0-9]$ ]]; then
                # Version already includes prerelease suffix
                :
            else
                if ! new_version=$(get_next_prerelease "$new_version"); then
                    exit 1
                fi
            fi
        fi
    elif [[ "$VERSION_TYPE" == "prerelease-only" ]]; then
        # Just increment prerelease number without bumping base version
        # Check if current version is already a prerelease
        if [[ "$current_version" =~ -prerelease[0-9]+$ ]]; then
            # Extract base version from current prerelease (handle both 1-digit and 2-digit)
            local base_version=$(echo "$current_version" | sed 's/v//' | sed 's/-prerelease[0-9]*$//')
            new_version=$(get_next_prerelease "$base_version")
        else
            # Current version is not prerelease, make it first prerelease of current version
            local base_version=$(echo "$current_version" | sed 's/v//')
            new_version="${base_version}-prerelease01"
        fi
    else
        # Increment based on type
        new_version=$(increment_version "$current_version" "$VERSION_TYPE")

        # Handle prerelease for version bumps
        if [[ "$PRERELEASE" == true ]]; then
            if ! new_version=$(get_next_prerelease "$new_version"); then
                exit 1
            fi
        fi
    fi

    # Check if version already exists
    if ! check_version_exists "$new_version" "${modules[@]}"; then
        exit 1
    fi

    # Show tags that will be created
    echo "Tags to be created:"
    for module in "${modules[@]}"; do
        local rel_path=$(get_relative_path "$module" "$REPO_ROOT")
        if [[ "$rel_path" == "." ]]; then
            echo "  - v$new_version"
        else
            echo "  - $rel_path/v$new_version"
        fi
    done
    echo

    if [[ "$DRY_RUN" == true ]]; then
        log_success "Dry run completed successfully!"
        exit 0
    fi

    # Process each module with the same version
    echo "Creating and pushing tags:"
    local failed_modules=()
    for module in "${modules[@]}"; do
        if ! create_module_release "$module" "$new_version"; then
            failed_modules+=("$module")
        fi
    done

    # Summary
    if [[ ${#failed_modules[@]} -eq 0 ]]; then
        echo
        log_success "All modules released successfully!"
    else
        echo
        log_error "Failed to process ${#failed_modules[@]} module(s):"
        for failed in "${failed_modules[@]}"; do
            echo "  - $(get_relative_path "$failed" "$REPO_ROOT")"
        done
        exit 1
    fi
}

# Run main function
main "$@"
