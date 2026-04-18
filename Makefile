# Copyright (c) 2017-2023 VMware, Inc. or its affiliates
# SPDX-License-Identifier: Apache-2.0

all: | install-dependencies generate build

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
	# For tagging a release see the "Upgrade Release Checklist" document.
	$(eval VERSION := $(shell git describe --tags --abbrev=0))
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
