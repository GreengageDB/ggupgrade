# Copyright (c) 2017-2023 VMware, Inc. or its affiliates
# SPDX-License-Identifier: Apache-2.0

all: build

.DEFAULT_GOAL := all
MODULE_NAME=ggupgrade


LINUX_ENV := env GOOS=linux GOARCH=amd64
MAC_ENV := env GOOS=darwin GOARCH=amd64


# NOTE: goimports subsumes the standard formatting rules of gofmt, but gofmt is
#       more flexible(custom rules) so we leave it in for this reason.
format:
		goimports -l -w agent/ cli/ hub/ test/integration/ testutils/ utils/
		gofmt -l -w agent/ cli/ hub/ test/integration/ testutils/ utils/

unit integration acceptance test: export PATH := $(CURDIR):$(PATH)

.PHONY: unit
unit:
	go test -count=1 $(shell go list ./... | grep -v test/integration$$ | grep -v test/acceptance/ggupgrade$$ | grep -v test/acceptance/pg_upgrade$$)

.PHONY: integration
integration:
	go test -count=1 ./test/integration

.PHONY: acceptance
acceptance:
	go test -count=1 -timeout 1h15m -v ./test/acceptance/ggupgrade

# test runs all tests against the locally built ggupgrade binaries. Use -k to
# continue after failures.
.PHONY: test check
test check: unit integration acceptance

.PHONY: pg-upgrade-tests
pg-upgrade-tests:
	go test -count=1 -timeout 35m -v ./test/acceptance/pg_upgrade

.PHONY: coverage
coverage:
	@./scripts/show_coverage.sh

BUILD_ENV = $($(OS)_ENV)

.PHONY: install-dependencies generate build build_linux build_mac

# We don't have file dependencies for the following targets (meaning that they have to be rebuilt for each `make` command)
# because `go` command itself is a build tool, and it doesn't work with `make` recipes.
# (an example of a conflict would be tracking modules downloaded via `go mod download` with `make`,
#  or installing a different version of the tool with `go install`)

TOOLS_DIR = dev-bin

$(TOOLS_DIR):
	mkdir $(TOOLS_DIR)

# Setup $GOBIN and $PATH to point to installed modules
export GOBIN := $(CURDIR)/$(TOOLS_DIR)
export PATH  := $(PATH):$(GOBIN)

# Fetch all used binaries/modules.
# Note, that these binaries should be present in the `tools.go` file so their version can be recoreded in `mod.go`.
# When adding a new tool, add its module to `tools.go`, then run `go mod tidy`, and finally put `go install` here.
install-dependencies: $(TOOLS_DIR)
	go mod download
	go install google.golang.org/protobuf/cmd/protoc-gen-go
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
	go install github.com/golang/mock/mockgen

generate:
	go generate	./idl
	go generate ./cli/bash

build:
	$(eval VERSION := $(GIT_VERSION))
	$(eval COMMIT := $(shell git rev-parse --short --verify HEAD))
	$(eval RELEASE=Dev Build)
	$(eval VERSION_LD_STR := -X 'github.com/GreengageDB/$(MODULE_NAME)/cli/commands.Version=$(VERSION)')
	$(eval VERSION_LD_STR += -X 'github.com/GreengageDB/$(MODULE_NAME)/cli/commands.Commit=$(COMMIT)')
	$(eval VERSION_LD_STR += -X 'github.com/GreengageDB/$(MODULE_NAME)/cli/commands.Release=$(RELEASE)')

	$(eval BUILD_FLAGS = -gcflags="all=-N -l")
	$(eval override BUILD_FLAGS += -ldflags "$(VERSION_LD_STR)")

	$(BUILD_ENV) go build -o ggupgrade $(BUILD_FLAGS) github.com/GreengageDB/ggupgrade/cmd/ggupgrade

build_linux: OS := LINUX
build_mac: OS := MAC
build_linux build_mac: build

BUILD_FLAGS = -gcflags="all=-N -l"
override BUILD_FLAGS += -ldflags "$(VERSION_LD_STR)"

oss-tarball: RELEASE=Open Source
oss-tarball: build tarball

TARBALL_NAME=ggupgrade.tar.gz

tarball:
	[ ! -d tarball ] && mkdir tarball
	# gather files
	cp ggupgrade tarball
	cp cli/bash/ggupgrade.bash tarball
	cp ggupgrade_config tarball
	cp open_source_licenses.txt tarball
	cp -r data-migration-scripts/ tarball/data-migration-scripts/
	# remove test files
	rm -r tarball/data-migration-scripts/README.md
	find -path "./tarball/data-migration-scripts/*/test" -type d -exec rm -r {} +
	# create tarball
	( cd tarball; tar czf ../$(TARBALL_NAME) . )
	sha256sum $(TARBALL_NAME) > CHECKSUM
	rm -r tarball

oss-rpm: RELEASE=Open Source
oss-rpm: NAME=Greengage Database Upgrade
oss-rpm: LICENSE=Apache 2.0
oss-rpm: oss-tarball rpm

rpm:
	[ ! -d rpm ] && mkdir rpm
	mkdir -p rpm/rpmbuild/{BUILD,RPMS,SOURCES,SPECS}
	cp $(TARBALL_NAME) rpm/rpmbuild/SOURCES
	cp ggupgrade.spec rpm/rpmbuild/SPECS/
	rpmbuild \
	--define "_topdir $${PWD}/rpm/rpmbuild" \
	--define "ggupgrade_version $(VERSION)" \
	--define "ggupgrade_rpm_release 1" \
	--define "release_type $(RELEASE)" \
	--define "license $(LICENSE)" \
	--define "summary $(NAME)" \
	-bb $${PWD}/rpm/rpmbuild/SPECS/ggupgrade.spec
	cp rpm/rpmbuild/RPMS/x86_64/ggupgrade-$(VERSION)*.rpm .
	rm -r rpm

#---------------------------------------------------------------------
# Packaging targets with changelog options (deb)
#---------------------------------------------------------------------

# Version resolution, first match wins:
#   1. PLUGIN_VERSION=X.Y.Z  - explicit override (release automation, source-archive builds)
#   2. git describe --tags   - normal checkout
#   3. .version              - git-archive substitution for source tarballs (see .gitattributes)
#   4. VERSION               - file generated by `make VERSION` (Docker clean stage)
RAW_VERSION := $(or $(PLUGIN_VERSION),$(shell git describe --tags 2>/dev/null),$(shell grep -v '\$$Format:' .version 2>/dev/null),$(shell cat VERSION 2>/dev/null))
GIT_VERSION := $(or $(shell printf '%s' '$(RAW_VERSION)' | perl -pe 's/(.*)-([0-9]+)-(g[0-9a-f]+)/\1+dev.\2.\3/' 2>/dev/null),$(RAW_VERSION))

VERSION_GOALS := all build debug build_linux build_mac install pkg pkg-deb pkg-info VERSION debian/changelog
ifneq (,$(filter $(VERSION_GOALS),$(or $(MAKECMDGOALS),all)))
ifeq ($(strip $(GIT_VERSION)),)
$(error version unknown - pass PLUGIN_VERSION=X.Y.Z, or build from a git checkout with tags / a release source archive)
endif
endif

# Metadata vars
GPROOT			:= /opt/greengagedb
PACKAGE_NAME	:= $(shell grep '^Package:' debian/control | head -1 | awk '{print $$2}')
MAINTAINER		:= $(shell grep '^Maintainer:' debian/control | sed 's/Maintainer: //')
DATE_RFC		:= $(shell date -R)
DISTRO_CODENAME := $(shell lsb_release -sc 2>/dev/null)
IS_RELEASE		:= $(if $(GIT_VERSION),$(if $(findstring +dev,$(GIT_VERSION)),no,yes))
BUILD_TYPE		:= $(if $(filter yes,$(IS_RELEASE)),Release build,Development build)
DEB_TOPDIR		?= $(CURDIR)/../deb-packages

# Generate for Dockerfile where .git is absent
VERSION :
	@echo "Update $@"
	@echo "$(GIT_VERSION)" > $@
	@cat $@

debian/changelog:
	@echo "$(PACKAGE_NAME) ($(GIT_VERSION)) $(DISTRO_CODENAME); urgency=low" > $@
	@echo "" >> $@
	@echo "  * $(BUILD_TYPE)" >> $@
	@echo "" >> $@
	@echo " -- $(MAINTAINER)  $(DATE_RFC)" >> $@

debian/install:
	@echo "$(PACKAGE_NAME)/* /" > $@

# Default packaging target
pkg : pkg-info pkg-deb

# Display package info
pkg-info :
	@echo "PACKAGE_NAME: $(PACKAGE_NAME)"
	@echo "MAINTAINER: $(MAINTAINER)"
	@echo "DATE_RFC: $(DATE_RFC)"
	@echo "GIT_VERSION: $(GIT_VERSION)"
	@echo "DISTRO_CODENAME: $(DISTRO_CODENAME)"
	@echo "IS_RELEASE: $(IS_RELEASE)"
	@echo "BUILD_TYPE: $(BUILD_TYPE)"

# Build Debian package
pkg-deb : debian/changelog debian/install
	@GPROOT="$(GPROOT)" PACKAGE_NAME="$(PACKAGE_NAME)" debuild --preserve-env -us -uc -b
	@mkdir -p $(DEB_TOPDIR)
	@find $(CURDIR)/../ -maxdepth 1 -type f \( -name "*.deb" \
											-o -name "*.ddeb" \
											-o -name "*.build" \
											-o -name "*.buildinfo" \
											-o -name "*.changes" \) \
											-exec mv -f {} $(DEB_TOPDIR)/ \;

.PHONY: debian/changelog debian/install pkg pkg-info pkg-deb

install:
	@test $${GOPATH?Error GOPATH not set}
	cp -f ggupgrade $(GOPATH)/bin/

# To lint, you must install golangci-lint via one of the supported methods
# listed at
#
#     https://github.com/golangci/golangci-lint#install
#
# DO NOT add the linter to the project dependencies in Gopkg.toml, as much as
# you may want to streamline this installation process, because
# 1. `go get` is an explicitly unsupported installation method for this utility,
#    much like it is for ggupgrade itself, and
# 2. adding it as a project dependency opens up the possibility of accidentally
#    vendoring GPL'd code.
.PHONY: lint
lint:
	golangci-lint run

clean:
		# Build artifacts
		rm -f ggupgrade
		# Test artifacts
		rm -rf /tmp/go-build*
		rm -rf /tmp/gexec_artifacts*
		# Code coverage files
		rm -rf /tmp/cover*
		rm -rf /tmp/unit*
		# Package artifacts
		rm -rf tarball
		rm -f $(TARBALL_NAME)
		rm -f CHECKSUM
		rm -rf rpm
		rm -f ggupgrade-$(VERSION)*.rpm
		# Generated files
		rm -f cli/bash/ggupgrade.bash
		rm -f idl/*.pb.go
		rm -f idl/mock_idl/*

# You can override these from the command line.
BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD)
GIT_URI ?= $(shell git ls-remote --get-url)

ifeq ($(GIT_URI),https://github.com/GreengageDB/ggupgrade.git)
ifeq ($(BRANCH),main)
	PIPELINE_NAME := ggupgrade
	FLY_TARGET := prod
endif
endif

# Concourse does not allow "/" in pipeline names
WORKSPACE ?= ~/workspace
BRANCH_NAME ?= $(shell git rev-parse --abbrev-ref HEAD | tr '/' ':')
export BRANCH_NAME
PIPELINE_NAME ?= ggupgrade:${BRANCH_NAME}
FLY_TARGET ?= cm

# YAML templating is used to switch between prod and dev pipelines. The
# environment variable JOB_TYPE is used to determine whether a dev or prod
# pipeline is generated. It is used when go generate runs our yaml parser.
ifeq ($(FLY_TARGET),prod)
pipeline pipeline7 functional-pipeline: export JOB_TYPE=prod
else
pipeline pipeline7 functional-pipeline: export JOB_TYPE=dev
endif

.PHONY: pipeline pipeline7 functional-pipeline expose-pipeline
pipeline pipeline7 functional-pipeline: export DUMP_PATH=${DUMP_PATH:-}
pipeline pipeline7 functional-pipeline: export 5X_GIT_USER=${5X_GIT_USER:-}
pipeline pipeline7 functional-pipeline: export 5X_GIT_BRANCH=${5X_GIT_BRANCH:-}
pipeline pipeline7 functional-pipeline: export 6X_GIT_USER=${6X_GIT_USER:-}
pipeline pipeline7 functional-pipeline: export 6X_GIT_BRANCH=${6X_GIT_BRANCH:-}
pipeline pipeline7 functional-pipeline: export 7X_GIT_USER=${7X_GIT_USER:-}
pipeline pipeline7 functional-pipeline: export 7X_GIT_BRANCH=${7X_GIT_BRANCH:-}
pipeline:
	mkdir -p ci/main/generated
	cat ci/main/pipeline/1_resources_anchors_groups.yml \
		ci/main/pipeline/2_build_lint.yml \
		ci/main/pipeline/3_ggupgrade_jobs.yml  \
		ci/main/pipeline/4_pg_upgrade_jobs.yml  \
		ci/main/pipeline/5_multi_host_ggupgrade_jobs.yml \
		ci/main/pipeline/6_upgrade_and_functional_jobs.yml \
		ci/main/pipeline/7_publish_rc.yml > ci/main/generated/template.yml
	PIPELINE_VERSION="6" go generate ./ci/main
	#NOTE-- make sure your ggupgrade-git-remote uses an https style git"
	#NOTE-- such as https://github.com/GreengageDB/ggupgrade.git"
	fly -t $(FLY_TARGET) set-pipeline -p $(PIPELINE_NAME) \
		-c ci/main/generated/pipeline.yml \
		-v ggupgrade-git-remote=$(GIT_URI) \
		-v ggupgrade-git-branch=$(BRANCH)

pipeline7:
	mkdir -p ci/main/generated
	cat ci/main/pipeline/1_resources_anchors_groups.yml \
		ci/main/pipeline/2_build_lint.yml \
		ci/main/pipeline/3_ggupgrade_jobs.yml  \
		ci/main/pipeline/4_pg_upgrade_jobs.yml  \
		ci/main/pipeline/5_multi_host_ggupgrade_jobs.yml \
		ci/main/pipeline/6_upgrade_and_functional_jobs.yml \
		ci/main/pipeline/7_publish_rc.yml > ci/main/generated/template.yml
	PIPELINE_VERSION="7" go generate ./ci/main
	#NOTE-- make sure your ggupgrade-git-remote uses an https style git"
	#NOTE-- such as https://github.com/GreengageDB/ggupgrade.git"
	fly -t $(FLY_TARGET) set-pipeline -p 7-$(PIPELINE_NAME) \
		-c ci/main/generated/pipeline.yml \
		-v ggupgrade-git-remote=$(GIT_URI) \
		-v ggupgrade-git-branch=$(BRANCH)

functional-pipeline:
	mkdir -p ci/functional/generated
	cat ci/functional/pipeline/1_resources_anchors_groups.yml \
		ci/functional/pipeline/2_generate_cluster.yml \
		ci/functional/pipeline/3_load_schema_data_migration_scripts.yml \
		ci/functional/pipeline/4_initialize_upgrade_cluster_validate.yml \
		ci/functional/pipeline/5_teardown_cluster.yml > ci/functional/generated/template.yml
	go generate ./ci/functional
	#NOTE-- make sure your ggupgrade-git-remote uses an https style git"
	#NOTE-- such as https://github.com/GreengageDB/ggupgrade.git"
	fly -t $(FLY_TARGET) set-pipeline -p $(PIPELINE_NAME) \
		-c ci/functional/generated/pipeline.yml \
		-v ggupgrade-git-remote=$(GIT_URI) \
		-v ggupgrade-git-branch=$(BRANCH)

expose-pipeline:
	fly --target $(FLY_TARGET) expose-pipeline --pipeline $(PIPELINE_NAME)
