# get rid of default behaviors, they're just noise
MAKEFLAGS += --no-builtin-rules
.SUFFIXES:

# pipefail is very easy to forget about, and sadly not the default.
# this changes that, and makes our target scripts more strict.
#
# make sure to include all this if overriding on the CLI, e.g.:
# 	DO:      make SHELL='bash -eoux pipefail'
# 	DO NOT:  make SHELL='bash -x'
# as otherwise you will ignore all errors.
SHELL = /bin/bash -e -u -o pipefail

default: help

# ###########################################
#                TL;DR DOCS:
# ###########################################
# - Targets should never, EVER be *actual source files*.
#   Always use book-keeping files in $(BUILD).
#   Otherwise e.g. changing git branches could confuse Make about what it needs to do.
# - Similarly, prerequisites should be those book-keeping files,
#   not source files that are prerequisites for book-keeping.
#   e.g. depend on .build/fmt, not $(ALL_SRC), and not both.
# - Be strict and explicit about prerequisites / order of execution / etc.
# - Test your changes with `-j 27 --output-sync` or something!
# - Test your changes with `make -d ...`!  It should be reasonable!

# temporary build products and book-keeping targets that are always good to / safe to clean.
BUILD := .build
# bins that are `make clean` friendly, i.e. they build quickly and do not require new downloads.
# in particular this should include goimports, as it changes based on which version of go compiles it,
# and few know to do more than `make clean`.
BIN := $(BUILD)/bin
# relatively stable build products, e.g. tools.
# usually unnecessary to clean, and may require downloads to restore, so this folder is not automatically cleaned.
STABLE_BIN := .bin

# toolchain version we all use.
# this export ensures it is a precise version rather than a minimum.
# lint step ensures this matches other files.
GOWORK_TOOLCHAIN := $(word 2,$(shell grep 'toolchain' go.work))
export GOTOOLCHAIN ?= $(GOWORK_TOOLCHAIN)
ifneq ($(GOTOOLCHAIN),$(GOWORK_TOOLCHAIN))
# this can be useful for trying new/old versions, so don't block it
$(warning warning: your Go toolchain is explicitly set to GOTOOLCHAIN=$(GOTOOLCHAIN), ignoring go.work version $(GOWORK_TOOLCHAIN)...)
endif

# ====================================
# book-keeping files that are used to control sequencing.
#
# you should use these as prerequisites in almost all cases, not the source files themselves.
# these are defined in roughly the reverse order that they are executed, for easier reading.
#
# recipes and any other prerequisites are defined only once, further below.
# ====================================

# all bins depend on: $(BUILD)/lint
# note that vars that do not yet exist are empty, so any prerequisites defined below are ineffective here.
$(BUILD)/lint: $(BUILD)/fmt # lint will fail if fmt fails, so fmt first
$(BUILD)/proto-lint:
$(BUILD)/gomod-lint:
$(BUILD)/goversion-lint:
$(BUILD)/fmt: $(BUILD)/codegen # formatting must occur only after all other go-file-modifications are done
# $(BUILD)/copyright
# $(BUILD)/copyright: $(BUILD)/codegen # must add copyright to generated code, sometimes needs re-formatting
$(BUILD)/codegen: $(BUILD)/thrift $(BUILD)/protoc
$(BUILD)/thrift: $(BUILD)/go_mod_check
$(BUILD)/protoc: $(BUILD)/go_mod_check
$(BUILD)/go_mod_check:

# ====================================
# helper vars
# ====================================

# a literal space value, for makefile purposes.
# the full "trailing # one space after $(null)" is necessary for correct behavior,
# and this strategy works in both new and old versions of make, `SPACE +=` does not.
null  :=
SPACE := $(null) #
COMMA := ,

# set a V=1 env var for verbose output. V=0 (or unset) disables.
# this is used to make two verbose flags:
# - $Q, to replace ALL @ use, so CI can be reliably verbose
# - $(verbose), to forward verbosity flags to commands via `$(if $(verbose),-v)` or similar
#
# SHELL='bash -x' is useful too, but can be more confusing to understand.
V ?= 0
ifneq (0,$(V))
verbose := 1
Q :=
else
verbose :=
Q := @
endif

# and enforce ^ that rule: grep the makefile for line-starting @ use, error if any exist.
# limit to one match because multiple look too weird.
_BAD_AT_USE=$(shell grep -n -m1 '^\s*@' $(MAKEFILE_LIST))
ifneq (,$(_BAD_AT_USE))
$(warning Makefile cannot use @ to silence commands, use $$Q instead:)
$(warning found on line $(_BAD_AT_USE))
$(error fix that line and try again)
endif

# M1 macs may need to switch back to x86, until arm releases are available
EMULATE_X86 =
ifeq ($(shell uname -sm),Darwin arm64)
EMULATE_X86 = arch -x86_64
endif

PROJECT_ROOT = github.com/uber/cadence

# helper for executing bins that need other bins, just `$(BIN_PATH) the_command ...`
# I'd recommend not exporting this in general, to reduce the chance of accidentally using non-versioned tools.
BIN_PATH := PATH="$(abspath $(BIN)):$(abspath $(STABLE_BIN)):$$PATH"

# automatically gather all source files that currently exist.
# works by ignoring everything in the parens (and does not descend into matching folders) due to `-prune`,
# and everything else goes to the other side of the `-o` branch, which is `-print`ed.
# this is dramatically faster than a `find . | grep -v vendor` pipeline, and scales far better.
FRESH_ALL_SRC = $(shell \
	find . \
	\( \
		-path './vendor/*' \
		-o -path './idls/*' \
		-o -path './.build/*' \
		-o -path './.bin/*' \
		-o -path './.git/*' \
	\) \
	-prune \
	-o -name '*.go' \
	-type f \
	-print \
)
# most things can use a cached copy, e.g. all dependencies.
# this will not include any files that are created during a `make` run, e.g. via protoc,
# but that generally should not matter (e.g. dependencies are computed at parse time, so it
# won't affect behavior either way - choose the fast option).
#
# if you require a fully up-to-date list, e.g. for shell commands, use FRESH_ALL_SRC instead.
ALL_SRC := $(FRESH_ALL_SRC)
# as lint ignores generated code, it can use the cached copy in all cases
LINT_SRC := $(filter-out %_test.go ./.gen/%, $(ALL_SRC))

# ====================================
# $(BIN) targets
# ====================================

# downloads and builds a go-gettable tool, versioned by go.mod, and installs
# it into the build folder, named the same as the last portion of the URL.
#
# unfortunately go.work and `go list -modfile=sub/module/go.mod` seem to interact badly,
# and some versions complain about duplicates, while others simply block it outright.
# the good news is that you can just drop that and `cd` to the folder and it works.
define go_build_tool
$Q echo "building $(or $(2), $(notdir $(1))) from internal/tools/go.mod..."
$Q cd internal/tools; GOTOOLCHAIN=auto go build -mod=readonly -o ../../$(BIN)/$(or $(2), $(notdir $(1))) $(1)
endef

# same as go_build_tool, but uses our main module file, not the tools one.
# this is necessary / useful for tools that we are already importing in the repo, e.g. yarpc.
# versions here are checked to make sure the tools version matches the service version.
#
# this is an imperfect check as it only checks the top-level version.
# checks of other packages are handled in $(BUILD)/go_mod_check
define go_mod_build_tool
$Q echo "building $(or $(2), $(notdir $(1))) from go.mod..."
$Q ./scripts/check-gomod-version.sh $(1) $(if $(verbose),-v)
$Q GOTOOLCHAIN=auto go build -mod=readonly -o $(BIN)/$(or $(2), $(notdir $(1))) $(1)
endef

# utility target.
# use as an order-only prerequisite for targets that do not implicitly create these folders.
$(BIN) $(BUILD) $(STABLE_BIN):
	$Q mkdir -p $@

$(BIN)/thriftrw: go.mod go.work
	$(call go_mod_build_tool,go.uber.org/thriftrw)

$(BIN)/thriftrw-plugin-yarpc: go.mod go.work
	$(call go_mod_build_tool,go.uber.org/yarpc/encoding/thrift/thriftrw-plugin-yarpc)

$(BIN)/mockgen: internal/tools/go.mod go.work
	$(call go_build_tool,go.uber.org/mock/mockgen)

$(BIN)/mockery: internal/tools/go.mod go.work
	$(call go_build_tool,github.com/vektra/mockery/v2,mockery)

$(BIN)/enumer: internal/tools/go.mod go.work
	$(call go_build_tool,github.com/dmarkham/enumer)

# organizes imports and reformats
$(BIN)/gci: internal/tools/go.mod go.work
	$(call go_build_tool,github.com/daixiang0/gci)

# removes unused imports and reformats
$(BIN)/goimports: internal/tools/go.mod go.work
	$(call go_build_tool,golang.org/x/tools/cmd/goimports)

$(BIN)/gowrap: go.mod go.work
	$(call go_build_tool,github.com/hexdigest/gowrap/cmd/gowrap)

$(BIN)/revive: internal/tools/go.mod go.work
	$(call go_build_tool,github.com/mgechev/revive)

$(BIN)/nilaway: internal/tools/go.mod go.work
	$(call go_build_tool,go.uber.org/nilaway/cmd/nilaway,nilaway)

$(BIN)/protoc-gen-gogofast: go.mod go.work | $(BIN)
	$(call go_mod_build_tool,github.com/gogo/protobuf/protoc-gen-gogofast)

$(BIN)/protoc-gen-yarpc-go: go.mod go.work | $(BIN)
	$(call go_mod_build_tool,go.uber.org/yarpc/encoding/protobuf/protoc-gen-yarpc-go)

$(BUILD)/go_mod_check: go.mod internal/tools/go.mod go.work
	$Q # generated == used is occasionally important for gomock / mock libs in general.  this is not a definite problem if violated though.
	$Q ./scripts/check-gomod-version.sh github.com/golang/mock/gomock $(if $(verbose),-v)
	$Q touch $@

# copyright header checker/writer.  only requires stdlib, so no other dependencies are needed.
# $(BIN)/copyright: cmd/tools/copyright/licensegen.go
# 	$Q go build -o $@ ./cmd/tools/copyright/licensegen.go

# https://docs.buf.build/
# changing BUF_VERSION will automatically download and use the specified version.
BUF_VERSION = 0.36.0
OS = $(shell uname -s)
ARCH = $(shell $(EMULATE_X86) uname -m)
BUF_URL = https://github.com/bufbuild/buf/releases/download/v$(BUF_VERSION)/buf-$(OS)-$(ARCH)
# use BUF_VERSION_BIN as a bin prerequisite, not "buf", so the correct version will be used.
# otherwise this must be a .PHONY rule, or the buf bin / symlink could become out of date.
BUF_VERSION_BIN = buf-$(BUF_VERSION)
$(STABLE_BIN)/$(BUF_VERSION_BIN): | $(STABLE_BIN)
	$Q echo "downloading buf $(BUF_VERSION)"
	$Q curl -sSL $(BUF_URL) -o $@
	$Q chmod +x $@

# https://www.grpc.io/docs/languages/go/quickstart/
# protoc-gen-gogofast (yarpc) are versioned via tools.go + go.mod (built above) and will be rebuilt as needed.
# changing PROTOC_VERSION will automatically download and use the specified version
PROTOC_VERSION = 3.14.0
PROTOC_URL = https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-$(subst Darwin,osx,$(OS))-$(ARCH).zip
# the zip contains an /include folder that we need to use to learn the well-known types
PROTOC_UNZIP_DIR = $(STABLE_BIN)/protoc-$(PROTOC_VERSION)-zip
# use PROTOC_VERSION_BIN as a bin prerequisite, not "protoc", so the correct version will be used.
# otherwise this must be a .PHONY rule, or the buf bin / symlink could become out of date.
PROTOC_VERSION_BIN = protoc-$(PROTOC_VERSION)
$(STABLE_BIN)/$(PROTOC_VERSION_BIN): | $(STABLE_BIN)
	$Q echo "downloading protoc $(PROTOC_VERSION): $(PROTOC_URL)"
	$Q # recover from partial success
	$Q rm -rf $(STABLE_BIN)/protoc.zip $(PROTOC_UNZIP_DIR)
	$Q # download, unzip, copy to a normal location
	$Q curl -sSL $(PROTOC_URL) -o $(STABLE_BIN)/protoc.zip
	$Q unzip -q $(STABLE_BIN)/protoc.zip -d $(PROTOC_UNZIP_DIR)
	$Q cp $(PROTOC_UNZIP_DIR)/bin/protoc $@

# checks that the idl submodule points to a commit on master, and that it matches the go module (which must be a pseudo version).
# this is only used in an explicit CI step, because it's expected to fail when developing.
#
# `git ls-tree HEAD idls` is selected because this only cares about the committed/checked-out target,
# not whatever the current status is, because only the committed value will exist for others.
#
# and last but not least: this avoids using `go` to make this check take only a couple seconds in CI,
# so the whole docker container doesn't have to be prepared.
.idl-status:
	$(Q) cd idls && \
		SUBMODULE_COMMIT=$$(git rev-parse HEAD) && \
		BRANCH_INFO=$$(git branch -r --contains "$$SUBMODULE_COMMIT" | head -n1) && \
		if ! git branch -r --contains "$$SUBMODULE_COMMIT" | grep -q "origin/master"; then \
			echo "Error: Submodule commit $$SUBMODULE_COMMIT belongs to $$BRANCH_INFO, not to master branch" && \
			exit 1; \
		fi
	$(Q) idlsha=$$(git ls-tree HEAD idls | awk '{print substr($$3,0,12)}') && \
		gosha=$$(grep github.com/uber/cadence-idl go.mod | tr '-' '\n' | tail -n1) && \
		if [[ "$$idlsha" != "$$gosha" ]]; then \
			echo "IDL submodule sha ($$idlsha) does not match go module sha ($$gosha)." >&2 && \
			echo "Make sure the IDL PR has been merged, and this PR is updated, before merging here." >&2 && \
			exit 1; \
		fi


# ====================================
# Codegen targets
# ====================================

# IDL submodule must be populated, or files will not exist -> prerequisites will be wrong -> build will fail.
# Because it must exist before the makefile is parsed, this cannot be done automatically as part of a build.
# Instead: call this func in targets that require the submodule to exist, so that target will not be built.
#
# THRIFT_FILES is just an easy identifier for "the submodule has files", others would work fine as well.
define ensure_idl_submodule
$(if $(THRIFT_FILES),,$(error idls/ submodule must exist, or build will fail.  Run `git submodule update --init` and try again))
endef

# codegen is done when thrift and protoc are done
$(BUILD)/codegen: $(BUILD)/thrift $(BUILD)/protoc | $(BUILD)
	$Q touch $@

THRIFT_FILES := $(shell find idls -name '*.thrift')
# book-keeping targets to build.  one per thrift file.
# idls/thrift/thing.thrift -> .build/thing.thrift
# the reverse is done in the recipe.
THRIFT_GEN := $(subst idls/thrift/,.build/,$(THRIFT_FILES))

# thrift is done when all sub-thrifts are done
$(BUILD)/thrift: $(THRIFT_GEN) | $(BUILD)
	$(call ensure_idl_submodule)
	$Q touch $@

# how to generate each thrift book-keeping file.
#
# note that each generated file depends on ALL thrift files - this is necessary because they can import each other.
# as --no-recurse is specified, these can be done in parallel, since output files will not overwrite each other.
$(THRIFT_GEN): $(THRIFT_FILES) $(BIN)/thriftrw $(BIN)/thriftrw-plugin-yarpc | $(BUILD)
	$Q echo 'thriftrw for $(subst .build/,idls/thrift/,$@)...'
	$Q $(BIN_PATH) $(BIN)/thriftrw \
		--plugin=yarpc \
		--pkg-prefix=$(PROJECT_ROOT)/.gen/go \
		--out=.gen/go \
		--no-recurse \
		$(subst .build/,idls/thrift/,$@)
	$Q touch $@

PROTO_ROOT := proto
# output location is defined by `option go_package` in the proto files, all must stay in sync with this
PROTO_OUT := .gen/proto
PROTO_FILES = $(shell find -L ./$(PROTO_ROOT) -name "*.proto" | grep -v "persistenceblobs" | grep -v public)
PROTO_DIRS = $(sort $(dir $(PROTO_FILES)))

# protoc splits proto files into directories, otherwise protoc-gen-gogofast is complaining about inconsistent package
# import paths due to multiple packages being compiled at once.
#
# After compilation files are moved to final location, as plugins adds additional path based on proto package.
$(BUILD)/protoc: $(PROTO_FILES) $(STABLE_BIN)/$(PROTOC_VERSION_BIN) $(BIN)/protoc-gen-gogofast $(BIN)/protoc-gen-yarpc-go | $(BUILD)
	$(call ensure_idl_submodule)
	$Q mkdir -p $(PROTO_OUT)
	$Q echo "protoc..."
	$Q chmod +x $(STABLE_BIN)/$(PROTOC_VERSION_BIN)
	$Q $(foreach PROTO_DIR,$(PROTO_DIRS),$(EMULATE_X86) $(STABLE_BIN)/$(PROTOC_VERSION_BIN) \
		--plugin $(BIN)/protoc-gen-gogofast \
		--plugin $(BIN)/protoc-gen-yarpc-go \
		-I=$(PROTO_ROOT)/public \
		-I=$(PROTO_ROOT)/internal \
		-I=$(PROTOC_UNZIP_DIR)/include \
		--gogofast_out=Mgoogle/protobuf/duration.proto=github.com/gogo/protobuf/types,Mgoogle/protobuf/field_mask.proto=github.com/gogo/protobuf/types,Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,Mgoogle/protobuf/wrappers.proto=github.com/gogo/protobuf/types,paths=source_relative:$(PROTO_OUT) \
		--yarpc-go_out=$(PROTO_OUT) \
		$$(find $(PROTO_DIR) -name '*.proto');\
	)
	$Q # This directory exists for local/github_actions but not for docker builds.
	$Q if [ -d "$(PROTO_OUT)/uber/cadence" ]; then \
		cp -R $(PROTO_OUT)/uber/cadence/* $(PROTO_OUT)/; \
		rm -r $(PROTO_OUT)/uber; \
	   fi
	$Q touch $@

# ====================================
# Rule-breaking targets intended ONLY for special cases with no good alternatives.
# ====================================

# used to bypass checks when building binaries.
# this is primarily intended for docker image builds, but can be used to skip things locally.
.PHONY: .just-build
.just-build: | $(BUILD)
	touch $(BUILD)/just-build

# ====================================
# other intermediates
# ====================================

$(BUILD)/proto-lint: $(PROTO_FILES) $(STABLE_BIN)/$(BUF_VERSION_BIN) | $(BUILD)
	$Q cd $(PROTO_ROOT) && ../$(STABLE_BIN)/$(BUF_VERSION_BIN) lint
	$Q touch $@

# lints that go modules are as expected, e.g. parent does not import submodule.
# tool builds that need to be in sync with the parent are partially checked through go_mod_build_tool, but should probably be checked here too
$(BUILD)/gomod-lint: go.mod internal/tools/go.mod common/archiver/gcloud/go.mod | $(BUILD)
	$Q echo "checking for direct submodule dependencies in root go.mod..."
	$Q ( \
		MAIN_MODULE=$$(grep "^module " go.mod | awk '{print $$2}'); \
		SUBMODULES=$$(find . -type f -name "go.mod" -not -path "./go.mod" -not -path "./idls/*" -exec dirname {} \; | sed 's|^\./||'); \
		for submodule in $$SUBMODULES; do \
			submodule_path="$$MAIN_MODULE/$$submodule"; \
			if grep -q "$$submodule_path" go.mod; then \
				echo "ERROR: Root go.mod directly depends on submodule: $$submodule_path" >&2; \
				exit 1; \
			fi; \
			echo "✓ No direct dependency on $$submodule"; \
		done; \
	)
	$Q touch $@

# note that LINT_SRC is fairly fake as a prerequisite.
# it's a coarse "you probably don't need to re-lint" filter, nothing more.
$(BUILD)/code-lint: $(LINT_SRC) $(BIN)/revive $(BIN)/nilaway | $(BUILD)
	$Q echo "lint..."
	$Q # non-optional vet checks.  unfortunately these are not currently included in `go test`'s default behavior.
	$Q go vet -copylocks ./... ./common/archiver/gcloud/...
	$Q $(BIN)/revive -config revive.toml -exclude './vendor/...' -exclude './.gen/...' -formatter stylish ./...
	$Q # look for go files with "//comments", and ignore "//go:build"-style directives ("grep -n" shows "file:line: //go:build" so the regex is a bit complex)
	$Q bad="$$(find . -type f -name '*.go' -not -path './idls/*' | xargs grep -n -E '^\s*//\S' | grep -E -v '^[^:]+:[^:]+:\s*//[a-z]+:[a-z]+' || true)"; \
		if [ -n "$$bad" ]; then \
		  echo "$$bad" >&2; \
		  echo 'non-directive comments must have a space after the "//"' >&2; \
		  exit 1; \
		fi
	$Q echo "nilaway check..."
	$Q GOTOOLCHAIN=go1.24.0 $(BIN)/nilaway -test=false github.com/uber/cadence/common/types/mapper/...
	$Q touch $@

$(BUILD)/goversion-lint: go.work Dockerfile docker/github_actions/Dockerfile${DOCKERFILE_SUFFIX}
	$Q echo "checking go version..."
	$Q # intentionally using go.work toolchain, as GOTOOLCHAIN is user-overridable
	$Q ./scripts/check-go-toolchain.sh $(GOWORK_TOOLCHAIN)
	$Q touch $@

# fmt and copyright are mutually cyclic with their inputs, so if a copyright header is modified:
# - copyright -> makes changes
# - fmt sees changes -> makes changes
# - now copyright thinks it needs to run again (but does nothing)
# - which means fmt needs to run again (but does nothing)
# and now after two passes it's finally stable, because they stopped making changes.
#
# this is not fatal, we can just run 2x.
# to be fancier though, we can detect when *both* are run, and re-touch the book-keeping files to prevent the second run.
# this STRICTLY REQUIRES that `copyright` and `fmt` are mutually stable, and that copyright runs before fmt.
# if either changes, this will need to change.
MAYBE_TOUCH_COPYRIGHT=

# use FRESH_ALL_SRC so it won't miss any generated files produced earlier.
$(BUILD)/fmt: $(ALL_SRC) $(BIN)/goimports $(BIN)/gci | $(BUILD)
	$Q echo "removing unused imports..."
	$Q # goimports thrashes on internal/tools, sadly.  just hide it.
	$Q $(BIN)/goimports -w $(filter-out ./internal/tools/tools.go,$(FRESH_ALL_SRC))
	$Q echo "grouping imports..."
	$Q $(BIN)/gci write --section standard --section 'Prefix(github.com/uber/cadence/)' --section default --section blank $(FRESH_ALL_SRC)
	$Q touch $@
# 	$Q $(MAYBE_TOUCH_COPYRIGHT)

# $(BUILD)/copyright: $(ALL_SRC) $(BIN)/copyright | $(BUILD)
# 	$(BIN)/copyright --verifyOnly
# 	$Q $(eval MAYBE_TOUCH_COPYRIGHT=touch $@)
# 	$Q touch $@

# ====================================
# developer-oriented targets
#
# many of these share logic with other intermediates, but are useful to make .PHONY for output on demand.
# as the Makefile is fast, it's reasonable to just delete the book-keeping file recursively make.
# this way the effort is shared with future `make` runs.
# ====================================

# "re-make" a target by deleting and re-building book-keeping target(s).
# the + is necessary for parallelism flags to be propagated
define remake
$Q rm -f $(addprefix $(BUILD)/,$(1))
$Q +$(MAKE) --no-print-directory $(addprefix $(BUILD)/,$(1))
endef

.PHONY: lint fmt copyright pr

# useful to actually re-run to get output again.
# reuse the intermediates for simplicity and consistency.
lint: ## (Re)run the linter
	$(call remake,proto-lint gomod-lint code-lint goversion-lint)

# intentionally not re-making, it's a bit slow and it's clear when it's unnecessary
fmt: $(BUILD)/fmt ## Run `gofmt` / organize imports / etc

# not identical to the intermediate target, but does provide the same codegen (or more).
# copyright: $(BIN)/copyright | $(BUILD) ## Update copyright headers
# 	$(BIN)/copyright
# 	$Q touch $(BUILD)/copyright

define make_quietly
$Q echo "make $1..."
$Q output=$$(mktemp); $(MAKE) $1 > $$output 2>&1 || ( cat $$output; echo -e '\nfailed `make $1`, check output above' >&2; exit 1)
endef

# pre-PR target to build and refresh everything
pr: ## Redo all codegen and basic checks, to ensure your PR will be able to run tests.  Recommended before opening a github PR
	$Q $(if $(verbose),$(MAKE) tidy,$(call make_quietly,tidy))
	$Q $(if $(verbose),$(MAKE) go-generate,$(call make_quietly,go-generate))
	$Q $(if $(verbose),$(MAKE) fmt,$(call make_quietly,fmt))
	$Q $(if $(verbose),$(MAKE) lint,$(call make_quietly,lint))
# 	$Q $(if $(verbose),$(MAKE) copyright,$(call make_quietly,copyright))

# ====================================
# binaries to build
# ====================================
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# normally, depend on lint, so a full build and check and codegen runs.
# docker builds though must *not* do this, and need to rely entirely on committed code.
ifeq (,$(wildcard $(BUILD)/just-build))
BINS_DEPEND_ON := $(BUILD)/lint
else
BINS_DEPEND_ON :=
$(warning !!!!! lint and codegen disabled, validations skipped !!!!!)
endif

BINS =
TOOLS =

BINS  += cadence-cassandra-tool
TOOLS += cadence-cassandra-tool
cadence-cassandra-tool: $(BINS_DEPEND_ON)
	$Q echo "compiling cadence-cassandra-tool with OS: $(GOOS), ARCH: $(GOARCH)"
	$Q ./scripts/build-with-ldflags.sh -o $@ cmd/tools/cassandra/main.go

BINS  += cadence-sql-tool
TOOLS += cadence-sql-tool
cadence-sql-tool: $(BINS_DEPEND_ON)
	$Q echo "compiling cadence-sql-tool with OS: $(GOOS), ARCH: $(GOARCH)"
	$Q ./scripts/build-with-ldflags.sh -o $@ cmd/tools/sql/main.go

BINS  += cadence
TOOLS += cadence
cadence: $(BINS_DEPEND_ON)
	$Q echo "compiling cadence with OS: $(GOOS), ARCH: $(GOARCH)"
	$Q ./scripts/build-with-ldflags.sh -o $@ cmd/tools/cli/main.go

BINS += cadence-server
cadence-server: $(BINS_DEPEND_ON)
	$Q echo "compiling cadence-server with OS: $(GOOS), ARCH: $(GOARCH)"
	$Q ./scripts/build-with-ldflags.sh -o $@ cmd/server/main.go

BINS += cadence-canary
cadence-canary: $(BINS_DEPEND_ON)
	$Q echo "compiling cadence-canary with OS: $(GOOS), ARCH: $(GOARCH)"
	$Q ./scripts/build-with-ldflags.sh -o $@ cmd/canary/main.go

BINS += sharddistributor-canary
sharddistributor-canary: $(BINS_DEPEND_ON)
	$Q echo "compiling sharddistributor-canary with OS: $(GOOS), ARCH: $(GOARCH)"
	$Q ./scripts/build-with-ldflags.sh -o $@ cmd/sharddistributor-canary/main.go

BINS += cadence-bench
cadence-bench: $(BINS_DEPEND_ON)
	$Q echo "compiling cadence-bench with OS: $(GOOS), ARCH: $(GOARCH)"
	$Q ./scripts/build-with-ldflags.sh -o $@ cmd/bench/main.go


BINS  += cadence-releaser
cadence-releaser: $(BINS_DEPEND_ON)
	$Q echo "compiling cadence-releaser with OS: $(GOOS), ARCH: $(GOARCH)"
	$Q ./scripts/build-with-ldflags.sh -o $@ cmd/tools/releaser/releaser.go

.PHONY: go-generate bins tools release clean

bins: $(BINS) ## Build all binaries, and any fast codegen needed (does not refresh wrappers or mocks)

tools: $(TOOLS)

go-generate: $(BIN)/mockgen $(BIN)/enumer $(BIN)/mockery  $(BIN)/gowrap ## Run `go generate` to regen mocks, enums, etc
	$Q echo "running go generate ./..., this takes a minute or more..."
	$Q # add our bins to PATH so `go generate` can find them
	$Q $(BIN_PATH) go generate $(if $(verbose),-v) ./...
	$Q $(MAKE) --no-print-directory fmt
# 	$Q echo "updating copyright headers"
# 	$Q $(MAKE) --no-print-directory copyright

release: ## Re-generate generated code and run tests
	$(MAKE) --no-print-directory go-generate
	$(MAKE) --no-print-directory test

build: ## `go build` all packages and tests (a quick compile check only, skips all other steps)
	$Q echo 'Building all packages and submodules...'
	$Q go build ./...
	$Q cd common/archiver/gcloud; go build ./...
	$Q cd cmd/server; go build ./...
	$Q # "tests" by building and then running `true`, and hides test-success output
	$Q echo 'Building all tests (~5x slower)...'
	$Q # intentionally not -race due to !race build tags
	$Q go test -exec /usr/bin/true ./... >/dev/null
	$Q cd common/archiver/gcloud; go test -exec /usr/bin/true ./... >/dev/null
	$Q cd cmd/server; go test -exec /usr/bin/true ./... >/dev/null

tidy: ## `go mod tidy` all packages
	$Q # tidy in dependency order
	$Q go mod tidy
	$Q cd common/archiver/gcloud; go mod tidy || (echo "failed to tidy gcloud plugin, try manually copying go.mod contents into common/archiver/gcloud/go.mod and rerunning" >&2; exit 1)
	$Q cd cmd/server; go mod tidy || (echo "failed to tidy main server module, try manually copying go.mod and common/archiver/gcloud/go.mod contents into cmd/server/go.mod and rerunning" >&2; exit 1)

clean: ## Clean build products and SQLite database
	rm -f $(BINS)
	rm -Rf $(BUILD)
	rm *.db
	$(if \
		$(wildcard $(STABLE_BIN)/*), \
		$(warning usually-stable build tools still exist, delete the $(STABLE_BIN) folder to rebuild them),)

# v----- not yet cleaned up -----v

.PHONY: git-submodules test bins build clean cover help

TOOLS_CMD_ROOT=./cmd/tools
INTEG_TEST_ROOT=./host
INTEG_TEST_DIR=host
INTEG_TEST_XDC_ROOT=./host/xdc
INTEG_TEST_XDC_DIR=hostxdc
INTEG_TEST_NDC_ROOT=./host/ndc
INTEG_TEST_NDC_DIR=hostndc

# INTEG_TEST_ROOT requires to use test flags that are not used in other sub directories
# go test require all test package support the flags that are used in the test
# So INTEG_TEST_DIRS contains all test directories of INTEG_TEST_ROOT that are not using test flags
INTEG_TEST_DIRS=$$(go list $(INTEG_TEST_ROOT)/... | grep -v $(INTEG_TEST_XDC_ROOT) | grep -v $(INTEG_TEST_NDC_ROOT) | grep -v $(INTEG_TEST_ROOT)$$)

# Opt out folders that shouldn't be run as part of unit tests such as integration tests, simulations.
# Syntax: "folder1% folder2%" # space separated list of folders to opt out
OPT_OUT_TEST_FOLDERS=./simulation%

TEST_TIMEOUT ?= 20m
TEST_ARG ?= -race $(if $(verbose),-v) -timeout $(TEST_TIMEOUT)

# TODO to be consistent, use nosql as PERSISTENCE_TYPE and cassandra PERSISTENCE_PLUGIN
# file names like integ_cassandra__cover should become integ_nosql_cassandra_cover
# for https://github.com/uber/cadence/issues/3514
PERSISTENCE_TYPE ?= cassandra
TEST_RUN_COUNT ?= 1
ifdef TEST_TAG
override TEST_TAG := -tags $(TEST_TAG)
endif

# all directories with *_test.go files in them (exclude host/xdc)
TEST_DIRS := $(filter-out $(INTEG_TEST_XDC_ROOT)%, $(sort $(dir $(filter %_test.go,$(ALL_SRC)))))
# all tests other than end-to-end integration test fall into the pkg_test category.
# ?= allows passing specific (space-separated) dirs for faster testing
PKG_TEST_DIRS ?= $(filter-out $(INTEG_TEST_ROOT)% $(OPT_OUT_TEST_FOLDERS), $(TEST_DIRS))

# Code coverage output files
COVER_ROOT                      := $(BUILD)/coverage
UNIT_COVER_FILE                 := $(COVER_ROOT)/unit_cover.out

INTEG_COVER_FILE                := $(COVER_ROOT)/integ_$(PERSISTENCE_TYPE)_$(PERSISTENCE_PLUGIN)_cover.out
INTEG_COVER_FILE_CASS           := $(COVER_ROOT)/integ_cassandra__cover.out
INTEG_COVER_FILE_MYSQL          := $(COVER_ROOT)/integ_sql_mysql_cover.out
INTEG_COVER_FILE_POSTGRES       := $(COVER_ROOT)/integ_sql_postgres_cover.out
INTEG_COVER_FILE_ETCD           := $(COVER_ROOT)/integ_etcd_cover.out

INTEG_NDC_COVER_FILE            := $(COVER_ROOT)/integ_ndc_$(PERSISTENCE_TYPE)_$(PERSISTENCE_PLUGIN)_cover.out
INTEG_NDC_COVER_FILE_CASS       := $(COVER_ROOT)/integ_ndc_cassandra__cover.out
INTEG_NDC_COVER_FILE_MYSQL      := $(COVER_ROOT)/integ_ndc_sql_mysql_cover.out
INTEG_NDC_COVER_FILE_POSTGRES   := $(COVER_ROOT)/integ_ndc_sql_postgres_cover.out

# Need the following option to have integration tests
# count towards coverage. godoc below:
# -coverpkg pkg1,pkg2,pkg3
#   Apply coverage analysis in each test to the given list of packages.
#   The default is for each test to analyze only the package being tested.
#   Packages are specified as import paths.
COVER_PKGS = client common host service tools
# pkg -> pkg/... -> github.com/uber/cadence/pkg/... -> join with commas
GOCOVERPKG_ARG := -coverpkg="$(subst $(SPACE),$(COMMA),$(addprefix $(PROJECT_ROOT)/,$(addsuffix /...,$(COVER_PKGS))))"

# iterates over a list of dirs and runs go test on each one, collecting errors as it runs.
# this is primarily written because it's a verbose bit of boilerplate, until we switch to `go test ./...` where possible.
# CAUTION: when changing to `go test ./...`, note that this DOES NOT test submodules.  Those must be run separately.
define looptest
$Q FAIL=""; for dir in $1; do \
	go test $(TEST_ARG) -coverprofile=$@ "$$dir" $(TEST_TAG) 2>&1 | tee -a test.log || FAIL="$$FAIL $$dir"; \
done; test -z "$$FAIL" || (echo "Failed packages; $$FAIL"; exit 1)
endef

test: ## Build and run all tests locally
	$Q rm -f test
	$Q rm -f test.log
	$Q echo Running special test cases without race detector:
	$Q go test -v ./cmd/server/cadence/
	$Q $(call looptest,$(PKG_TEST_DIRS))

test_dirs:
	echo $(PKG_TEST_DIRS)

test_e2e:
	$Q rm -f test
	$Q rm -f test.log
	$Q $(call looptest,$(INTEG_TEST_ROOT))

# need to run end-to-end xdc tests with race detector off because of ringpop bug causing data race issue
test_e2e_xdc:
	$Q rm -f test
	$Q rm -f test.log
	$Q $(call looptest,$(INTEG_TEST_XDC_ROOT))

cover_profile:
	$Q mkdir -p $(BUILD)
	$Q mkdir -p $(COVER_ROOT)
	$Q echo "mode: atomic" > $(UNIT_COVER_FILE)

	$Q echo Running special test cases without race detector:
	$Q go test ./cmd/server/cadence/
	$Q echo Running package tests:
	$Q go test $(PKG_TEST_DIRS) $(TEST_ARG) -coverprofile=$(UNIT_COVER_FILE)

cover_integration_profile:
	$Q mkdir -p $(BUILD)
	$Q mkdir -p $(COVER_ROOT)
	$Q echo "mode: atomic" > $(INTEG_COVER_FILE)

	$Q echo Running integration test with $(PERSISTENCE_TYPE) $(PERSISTENCE_PLUGIN)
	$Q mkdir -p $(BUILD)/$(INTEG_TEST_DIR)
	$Q time go test $(INTEG_TEST_DIRS) $(TEST_ARG) $(TEST_TAG) $(GOCOVERPKG_ARG) -coverprofile=$(BUILD)/$(INTEG_TEST_DIR)/coverage.out || exit 1;
	$Q cat $(BUILD)/$(INTEG_TEST_DIR)/coverage.out | grep -v "^mode: \w\+" >> $(INTEG_COVER_FILE)
	$Q time go test $(INTEG_TEST_ROOT) $(TEST_ARG) $(TEST_TAG) -persistenceType=$(PERSISTENCE_TYPE) -sqlPluginName=$(PERSISTENCE_PLUGIN) $(GOCOVERPKG_ARG) -coverprofile=$(BUILD)/$(INTEG_TEST_DIR)/coverage.out || exit 1;
	$Q cat $(BUILD)/$(INTEG_TEST_DIR)/coverage.out | grep -v "^mode: \w\+" >> $(INTEG_COVER_FILE)

integration_tests_etcd:
	$Q mkdir -p $(BUILD)
	$Q mkdir -p $(COVER_ROOT)
	$Q echo "mode: atomic" > $(INTEG_COVER_FILE_ETCD)

	$Q echo "Running integration tests with etcd"
	$Q mkdir -p $(BUILD)/$(INTEG_TEST_DIR)
	$Q (ETCD_TEST_DIRS=$$(find . -name "*_test.go" -exec grep -l "testhelper.SetupStoreTestCluster\|testflags.RequireEtcd" {} \; | xargs -n1 dirname | sort | uniq); \
		echo "Found etcd test directories:"; \
		echo "$$ETCD_TEST_DIRS"; \
		echo "Using ETCD_ENDPOINTS='$(ETCD_ENDPOINTS)'"; \
		ETCD=1 ETCD_ENDPOINTS="$(ETCD_ENDPOINTS)" time go test $$ETCD_TEST_DIRS $(TEST_ARG) $(TEST_TAG) -coverprofile=$(BUILD)/$(INTEG_TEST_DIR)/coverage.out || exit 1)
	$Q cat $(BUILD)/$(INTEG_TEST_DIR)/coverage.out | grep -v "^mode: \w\+" >> $(INTEG_COVER_FILE_ETCD)

cover_ndc_profile:
	$Q mkdir -p $(BUILD)
	$Q mkdir -p $(COVER_ROOT)
	$Q echo "mode: atomic" > $(INTEG_NDC_COVER_FILE)

	$Q echo Running integration test for 3+ dc with $(PERSISTENCE_TYPE) $(PERSISTENCE_PLUGIN)
	$Q mkdir -p $(BUILD)/$(INTEG_TEST_NDC_DIR)
	$Q time go test -timeout $(TEST_TIMEOUT) $(INTEG_TEST_NDC_ROOT) $(TEST_TAG) -persistenceType=$(PERSISTENCE_TYPE) -sqlPluginName=$(PERSISTENCE_PLUGIN) $(GOCOVERPKG_ARG) -coverprofile=$(BUILD)/$(INTEG_TEST_NDC_DIR)/coverage.out -count=$(TEST_RUN_COUNT) || exit 1;
	$Q cat $(BUILD)/$(INTEG_TEST_NDC_DIR)/coverage.out | grep -v "^mode: \w\+" | grep -v "mode: set" >> $(INTEG_NDC_COVER_FILE)

$(COVER_ROOT)/cover.out: $(UNIT_COVER_FILE) $(INTEG_COVER_FILE_CASS) $(INTEG_COVER_FILE_MYSQL) $(INTEG_COVER_FILE_POSTGRES) $(INTEG_NDC_COVER_FILE_CASS) $(INTEG_NDC_COVER_FILE_MYSQL) $(INTEG_NDC_COVER_FILE_POSTGRES)
	$Q echo "mode: atomic" > $(COVER_ROOT)/cover.out
	cat $(UNIT_COVER_FILE) | grep -v "^mode: \w\+" | grep -vP ".gen|_generated|[Mm]ock[s]?" >> $(COVER_ROOT)/cover.out
	cat $(INTEG_COVER_FILE_CASS) | grep -v "^mode: \w\+" | grep -vP ".gen|_generated|[Mm]ock[s]?" >> $(COVER_ROOT)/cover.out
	cat $(INTEG_COVER_FILE_MYSQL) | grep -v "^mode: \w\+" | grep -vP ".gen|_generated|[Mm]ock[s]?" >> $(COVER_ROOT)/cover.out
	cat $(INTEG_COVER_FILE_POSTGRES) | grep -v "^mode: \w\+" | grep -vP ".gen|_generated|[Mm]ock[s]?" >> $(COVER_ROOT)/cover.out
	cat $(INTEG_NDC_COVER_FILE_CASS) | grep -v "^mode: \w\+" | grep -vP ".gen|_generated|[Mm]ock[s]?" >> $(COVER_ROOT)/cover.out
	cat $(INTEG_NDC_COVER_FILE_MYSQL) | grep -v "^mode: \w\+" | grep -vP ".gen|_generated|[Mm]ock[s]?" >> $(COVER_ROOT)/cover.out
	cat $(INTEG_NDC_COVER_FILE_POSTGRES) | grep -v "^mode: \w\+" | grep -vP ".gen|_generated|[Mm]ock[s]?" >> $(COVER_ROOT)/cover.out
	cat $(INTEG_COVER_FILE_ETCD) | grep -v "^mode: \w\+" | grep -vP ".gen|_generated|[Mm]ock[s]?" >> $(COVER_ROOT)/cover.out

cover: $(COVER_ROOT)/cover.out
	go tool cover -html=$(COVER_ROOT)/cover.out;

install-schema: cadence-cassandra-tool
	$Q echo installing schema
	./cadence-cassandra-tool create -k cadence --rf 1
	./cadence-cassandra-tool -k cadence setup-schema -v 0.0
	./cadence-cassandra-tool -k cadence update-schema -d ./schema/cassandra/cadence/versioned
	./cadence-cassandra-tool create -k cadence_visibility --rf 1
	./cadence-cassandra-tool -k cadence_visibility setup-schema -v 0.0
	./cadence-cassandra-tool -k cadence_visibility update-schema -d ./schema/cassandra/visibility/versioned
	$Q echo installed schema

install-schema-mysql: cadence-sql-tool
	./cadence-sql-tool --user root --pw cadence create --db cadence
	./cadence-sql-tool --user root --pw cadence --db cadence setup-schema -v 0.0
	./cadence-sql-tool --user root --pw cadence --db cadence update-schema -d ./schema/mysql/v8/cadence/versioned
	./cadence-sql-tool --user root --pw cadence create --db cadence_visibility
	./cadence-sql-tool --user root --pw cadence --db cadence_visibility setup-schema -v 0.0
	./cadence-sql-tool --user root --pw cadence --db cadence_visibility update-schema -d ./schema/mysql/v8/visibility/versioned

install-schema-multiple-mysql: cadence-sql-tool install-schema-es-v7
	./cadence-sql-tool --user root --pw cadence create --db cadence0
	./cadence-sql-tool --user root --pw cadence --db cadence0 setup-schema -v 0.0
	./cadence-sql-tool --user root --pw cadence --db cadence0 update-schema -d ./schema/mysql/v8/cadence/versioned
	./cadence-sql-tool --user root --pw cadence create --db cadence1
	./cadence-sql-tool --user root --pw cadence --db cadence1 setup-schema -v 0.0
	./cadence-sql-tool --user root --pw cadence --db cadence1 update-schema -d ./schema/mysql/v8/cadence/versioned
	./cadence-sql-tool --user root --pw cadence create --db cadence2
	./cadence-sql-tool --user root --pw cadence --db cadence2 setup-schema -v 0.0
	./cadence-sql-tool --user root --pw cadence --db cadence2 update-schema -d ./schema/mysql/v8/cadence/versioned
	./cadence-sql-tool --user root --pw cadence create --db cadence3
	./cadence-sql-tool --user root --pw cadence --db cadence3 setup-schema -v 0.0
	./cadence-sql-tool --user root --pw cadence --db cadence3 update-schema -d ./schema/mysql/v8/cadence/versioned

install-schema-postgres: cadence-sql-tool
	./cadence-sql-tool -p 5432 -u postgres -pw cadence --pl postgres create --db cadence
	./cadence-sql-tool -p 5432 -u postgres -pw cadence --pl postgres --db cadence setup -v 0.0
	./cadence-sql-tool -p 5432 -u postgres -pw cadence --pl postgres --db cadence update-schema -d ./schema/postgres/cadence/versioned
	./cadence-sql-tool -p 5432 -u postgres -pw cadence --pl postgres create --db cadence_visibility
	./cadence-sql-tool -p 5432 -u postgres -pw cadence --pl postgres --db cadence_visibility setup-schema -v 0.0
	./cadence-sql-tool -p 5432 -u postgres -pw cadence --pl postgres --db cadence_visibility update-schema -d ./schema/postgres/visibility/versioned

install-schema-sqlite: cadence-sql-tool
	./cadence-sql-tool -pl sqlite --db cadence.db setup -v 0.0
	./cadence-sql-tool -pl sqlite --db cadence.db update-schema -d ./schema/sqlite/cadence/versioned
	./cadence-sql-tool -pl sqlite --db cadence_visibility.db setup -v 0.0
	./cadence-sql-tool -pl sqlite --db cadence_visibility.db update-schema -d ./schema/sqlite/visibility/versioned

install-schema-es-v7:
	curl -X PUT "http://127.0.0.1:9200/_template/cadence-visibility-template" -H 'Content-Type: application/json' -d @./schema/elasticsearch/v7/visibility/index_template.json
	curl -X PUT "http://127.0.0.1:9200/cadence-visibility-dev"

install-schema-es-v6:
	curl -X PUT "http://127.0.0.1:9200/_template/cadence-visibility-template" -H 'Content-Type: application/json' -d @./schema/elasticsearch/v6/visibility/index_template.json
	curl -X PUT "http://127.0.0.1:9200/cadence-visibility-dev"

install-schema-es-opensearch:
	curl -X PUT "https://127.0.0.1:9200/_template/cadence-visibility-template" -H 'Content-Type: application/json' -d @./schema/elasticsearch/os2/visibility/index_template.json -u admin:DevTestInitial123! --insecure
	curl -X PUT "https://127.0.0.1:9200/cadence-visibility-dev" -u admin:DevTestInitial123! --insecure

start: bins
	./cadence-server start

install-schema-xdc: cadence-cassandra-tool
	$Q echo Setting up cadence_cluster0 key space
	./cadence-cassandra-tool --ep 127.0.0.1 create -k cadence_cluster0 --rf 1
	./cadence-cassandra-tool --ep 127.0.0.1 -k cadence_cluster0 setup-schema -v 0.0
	./cadence-cassandra-tool --ep 127.0.0.1 -k cadence_cluster0 update-schema -d ./schema/cassandra/cadence/versioned
	./cadence-cassandra-tool --ep 127.0.0.1 create -k cadence_visibility_cluster0 --rf 1
	./cadence-cassandra-tool --ep 127.0.0.1 -k cadence_visibility_cluster0 setup-schema -v 0.0
	./cadence-cassandra-tool --ep 127.0.0.1 -k cadence_visibility_cluster0 update-schema -d ./schema/cassandra/visibility/versioned

	$Q echo Setting up cadence_cluster1 key space
	./cadence-cassandra-tool --ep 127.0.0.1 create -k cadence_cluster1 --rf 1
	./cadence-cassandra-tool --ep 127.0.0.1 -k cadence_cluster1 setup-schema -v 0.0
	./cadence-cassandra-tool --ep 127.0.0.1 -k cadence_cluster1 update-schema -d ./schema/cassandra/cadence/versioned
	./cadence-cassandra-tool --ep 127.0.0.1 create -k cadence_visibility_cluster1 --rf 1
	./cadence-cassandra-tool --ep 127.0.0.1 -k cadence_visibility_cluster1 setup-schema -v 0.0
	./cadence-cassandra-tool --ep 127.0.0.1 -k cadence_visibility_cluster1 update-schema -d ./schema/cassandra/visibility/versioned

	$Q echo Setting up cadence_cluster2 key space
	./cadence-cassandra-tool --ep 127.0.0.1 create -k cadence_cluster2 --rf 1
	./cadence-cassandra-tool --ep 127.0.0.1 -k cadence_cluster2 setup-schema -v 0.0
	./cadence-cassandra-tool --ep 127.0.0.1 -k cadence_cluster2 update-schema -d ./schema/cassandra/cadence/versioned
	./cadence-cassandra-tool --ep 127.0.0.1 create -k cadence_visibility_cluster2 --rf 1
	./cadence-cassandra-tool --ep 127.0.0.1 -k cadence_visibility_cluster2 setup-schema -v 0.0
	./cadence-cassandra-tool --ep 127.0.0.1 -k cadence_visibility_cluster2 update-schema -d ./schema/cassandra/visibility/versioned

start-xdc-cluster0: cadence-server
	./cadence-server --zone xdc_cluster0 start

start-xdc-cluster1: cadence-server
	./cadence-server --zone xdc_cluster1 start

start-xdc-cluster2: cadence-server
	./cadence-server --zone xdc_cluster2 start

start-canary: cadence-canary
	./cadence-canary start

start-sharddistributor-canary: sharddistributor-canary
	./sharddistributor-canary start

start-bench: cadence-bench
	./cadence-bench start

start-mysql: cadence-server
	./cadence-server --zone mysql start

start-postgres: cadence-server
	./cadence-server --zone postgres start

# broken up into multiple += so I can interleave comments.
# this all becomes a single line of output.
# you must not use single-quotes within the string in this var.
JQ_DEPS_AGE = jq '
# only deal with things with updates
JQ_DEPS_AGE += select(.Update)
# allow additional filtering, e.g. DEPS_FILTER='$(JQ_DEPS_ONLY_DIRECT)'
JQ_DEPS_AGE += $(DEPS_FILTER)
# add "days between current version and latest version"
JQ_DEPS_AGE += | . + {Age:(((.Update.Time | fromdate) - (.Time | fromdate))/60/60/24 | floor)}
# add "days between latest version and now"
JQ_DEPS_AGE += | . + {Available:((now - (.Update.Time | fromdate))/60/60/24 | floor)}
# 123 days: library 	old_version -> new_version
JQ_DEPS_AGE += | ([.Age, .Available] | max | tostring) + " days: " + .Path + "  \t" + .Version + " -> " + .Update.Version
JQ_DEPS_AGE += '
# remove surrounding quotes from output
JQ_DEPS_AGE += --raw-output

# exclude `"Indirect": true` dependencies.  direct ones have no "Indirect" key at all.
JQ_DEPS_ONLY_DIRECT = | select(has("Indirect") | not)

deps: ## Check for dependency updates, for things that are directly imported
	$Q make --no-print-directory DEPS_FILTER='$(JQ_DEPS_ONLY_DIRECT)' deps-all

deps-all: ## Check for all dependency updates
	$Q go list -u -m -json all \
		| $(JQ_DEPS_AGE) \
		| sort -n

help: ## Prints a help message showing any specially-commented targets
	$Q # print the high-value ones first, so they're more visible.  the "....." prefixes match the shell coloring chars
	$Q cat $(MAKEFILE_LIST) | grep -e "^[a-zA-Z_\-]*:.* ## .*" | awk 'BEGIN {FS = ":.*? ## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' | sort | grep -E '^.....(help|pr|bins)\b'
	$Q echo '-----------------------------------'
	$Q cat $(MAKEFILE_LIST) | grep -e "^[a-zA-Z_\-]*:.* ## .*" | awk 'BEGIN {FS = ":.*? ## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' | sort | grep -vE '^.....(help|pr|bins)\b'
