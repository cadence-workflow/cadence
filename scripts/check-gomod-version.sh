#!/usr/bin/env bash
set -eo pipefail

[[ $2 = "-v" ]] && set -x;

# go.work and `go list -modfile=...` seem to interact badly, and complain about duplicates.
# the good news is that you can just drop that and `cd` to the folder and it works.

# unfortunately Go has gotten very picky about module-vs-main-package and no longer
# reports version information for either with the same command.
# so this checks both formats for both module files.
#
# given history here, this will probably break again at some point.
# hopefully the "is module" split will remain the most complex reason :\


# current output used for this script:
#
# ❯ go list --mod=readonly -m github.com/golang/mock/gomock
# go: module github.com/golang/mock/gomock: not a known dependency
#
# ❯ go list -mod=readonly -f '{{ .Module }}' github.com/golang/mock/gomock
# github.com/golang/mock v1.6.0
#
# ❯ go list -mod=readonly -m golang.org/x/tools
# golang.org/x/tools v0.32.0
#
# ❯ go list -mod=readonly -f '{{ .Module }}' golang.org/x/tools
# cannot find module providing package golang.org/x/tools: import lookup disabled by -mod=readonly


is_module () {
    result="$(go list --mod=readonly -m "$1" 2>&1)"
    if [[ $result =~ "not a known dependency" ]]; then
        return 1
    fi
    return 0
}

get_module_version () {
    local importpath="$1"
    local failmsg="$2"
    if is_module "$1"; then
        if ! go list -mod=readonly -m "$importpath"; then
            >&2 echo "$failmsg"
            exit 1
        fi
    else
        if ! go list -mod=readonly -f '{{ .Module }}' "$importpath"; then
            >&2 echo "$failmsg"
            exit 1
        fi
    fi
}

gomod="$(get_module_version "$1" 'Error checking main go.mod.')"

cd internal/tools

toolmod="$(get_module_version "$1" 'Error checking tools go.mod, cd to internal/tools to modify it.')"

if [[ $gomod != "$toolmod" ]]; then
	>&2 echo "error: mismatched go.mod and tools go.mod"
    >&2 echo "ensure internal/tools/go.mod contains the same version as go.mod and try again:"
    >&2 echo -e "\t$gomod"
	exit 1
fi
