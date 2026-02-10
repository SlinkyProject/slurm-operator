##@ General

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Get the OS to set platform specific commands
UNAME_S ?= $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
	CP_FLAGS = -v -n
	SED = gsed
else
	CP_FLAGS = -v --update=none
	SED = sed
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

VERSION = $(shell cat ./VERSION)

.PHONY: version
version: ## Show current version.
	@echo VERSION=$(VERSION)

.PHONY: version-match
version-match: version ## Check if versions are consistent.
	@if [ -z "$$(echo $(VERSION) | grep -Eo "^[[:digit:]]+\.[[:digit:]]+\.[[:digit:]](-[[:alpha:]][[:alnum:]]*(\.[[:digit:]]+)?)?$$")" ]; then \
		echo "VERSION is not semver: $(VERSION)" ;\
		exit 1 ;\
	fi
	$(foreach chart, $(wildcard ./helm/**/Chart.yaml), $(SED) -i -E 's/version:[[:space:]]+.+$$/version: $(VERSION)/g' ${chart} ;)

##@ Build

.PHONY: all
all: build ## Build slurm-operator.

REGISTRY ?= slinky.slurm.net
BUILDER ?= project-v3-builder

.PHONY: build
build: build-images build-chart ## Build OCI packages.

.PHONY: build-images
build-images: ## Build container images.
	- $(CONTAINER_TOOL) buildx create --name $(BUILDER)
	REGISTRY=$(REGISTRY) VERSION=$(VERSION) $(CONTAINER_TOOL) buildx bake --builder=$(BUILDER)

.PHONY: build-chart
build-chart: helm-bin ## Build charts.
	$(foreach chart, $(wildcard ./helm/**/Chart.yaml), $(HELM) package --dependency-update helm/$(shell basename "$(shell dirname "${chart}")") ;)

.PHONY: push
push: push-images push-charts ## Push OCI packages.

.PHONY: push-images
push-images: build-images ## Push container images.
	REGISTRY=$(REGISTRY) VERSION=$(VERSION) $(CONTAINER_TOOL) buildx bake --builder=$(BUILDER) --push

.PHONY: push-charts
push-charts: build-chart ## Push OCI packages.
	$(foreach chart, $(wildcard ./*.tgz), $(HELM) push ${chart} oci://$(REGISTRY)/charts ;)

.PHONY: clean
clean: ## Clean executable files.
	- chmod -R -f u+w "$(LOCALBIN)" # make test installs files without write permissions.
	rm -rf "$(LOCALBIN)"
	rm -rf vendor/
	rm -f cover.out cover.html
	rm -f *.tgz
	- $(CONTAINER_TOOL) buildx rm $(BUILDER)

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | $(SED) "s/-$(3)$$//")" $(1) ;\
}
endef

# helm-install-plugin will 'helm plugin install' if missing or wrong version
# $1 - plugin name
# $2 - plugin urlgo
# $3 - plugin version (optional)
# verify_flag - added in the event that we are helm v4 or older due to https://helm.sh/docs/helm/helm_plugin_verify
#   being updated to be more secure, but libraries currently do not support it.
define helm-install-plugin
@{ \
set -e; \
if [ ! -x "$(HELM)" ]; then \
	echo "Helm binary not found at $(HELM). Run 'make helm-bin' first." ;\
	exit 1 ;\
fi ;\
helm_major="$$( $(HELM) version --short 2>/dev/null | sed -E 's/^v([0-9]+).*/\1/' )"; \
if [ "$${helm_major}" = "4" ]; then \
	verify_flag="--verify=false" ;\
else \
	verify_flag="" ;\
fi ;\
installed_version="$$( $(HELM) plugin list 2>/dev/null | awk '$$1=="$(1)" {print $$2}' )"; \
if [ -n "$${installed_version}" ]; then \
	if [ -n "$(3)" ] && [ "$${installed_version}" != "$(3)" ]; then \
		$(HELM) plugin install "$(2)" --version "$(3)" $${verify_flag} ;\
	fi ;\
else \
	if [ -n "$(3)" ]; then \
		$(HELM) plugin install "$(2)" --version "$(3)" $${verify_flag} ;\
	else \
		$(HELM) plugin install "$(2)" $${verify_flag} ;\
	fi ;\
fi ;\
}
endef

## Tool Binaries
KUBECTL ?= kubectl
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen-$(CONTROLLER_TOOLS_VERSION)
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk-$(OPERATOR_SDK_VERSION)
ENVTEST ?= $(LOCALBIN)/setup-envtest-$(ENVTEST_VERSION)
GOVULNCHECK ?= $(LOCALBIN)/govulncheck-$(GOVULNCHECK_VERSION)
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)
HELM_DOCS ?= $(LOCALBIN)/helm-docs-$(HELM_DOCS_VERSION)
PANDOC ?= $(LOCALBIN)/pandoc-$(PANDOC_VERSION)
HELM ?= $(LOCALBIN)/helm
HELM_CONFIG_HOME ?= $(LOCALBIN)/helm-config
HELM_CACHE_HOME ?= $(LOCALBIN)/helm-cache
HELM_DATA_HOME ?= $(LOCALBIN)/helm-data
HELM_PLUGINS ?= $(LOCALBIN)/helm-plugins
export HELM_CONFIG_HOME HELM_CACHE_HOME HELM_DATA_HOME HELM_PLUGINS

## Tool Versions
CONTROLLER_TOOLS_VERSION ?= v0.18.0
OPERATOR_SDK_VERSION ?= v1.41.1
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION ?= $(shell go list -m -f "{{ .Version }}" k8s.io/api | awk -F'[v.]' '{printf "1.%d", $$3}')
ENVTEST_VERSION ?= $(shell go list -m -f "{{ .Version }}" sigs.k8s.io/controller-runtime | awk -F'[v.]' '{printf "release-%d.%d", $$2, $$3}')
GOVULNCHECK_VERSION ?= latest
GOLANGCI_LINT_VERSION ?= v2.6.0
HELM_DOCS_VERSION ?= v1.14.2
PANDOC_VERSION ?= 3.7.0.2
HELM_VERSION ?= v4.1.0
HELM_UNITTEST_VERSION ?= v1.0.3

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: govulncheck-bin
govulncheck-bin: $(GOVULNCHECK) ## Download govulncheck locally if necessary.
$(GOVULNCHECK): $(LOCALBIN)
	$(call go-install-tool,$(GOVULNCHECK),golang.org/x/vuln/cmd/govulncheck,$(GOVULNCHECK_VERSION))

.PHONY: golangci-lint-bin
golangci-lint-bin: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	@if ! [ -f "$(GOLANGCI_LINT)" ]; then \
		wget -O- -nv https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(LOCALBIN) $(GOLANGCI_LINT_VERSION) ;\
		mv $(LOCALBIN)/golangci-lint $(GOLANGCI_LINT) ;\
	fi

.PHONY: helm-docs-bin
helm-docs-bin: $(HELM_DOCS) ## Download helm-docs locally if necessary.
$(HELM_DOCS): $(LOCALBIN)
	$(call go-install-tool,$(HELM_DOCS),github.com/norwoodj/helm-docs/cmd/helm-docs,$(HELM_DOCS_VERSION))

.PHONY: helm-bin
helm-bin: $(HELM) ## Download Helm v4 locally if necessary.
$(HELM): $(LOCALBIN)
	@if ! [ -f "$(HELM)" ]; then \
		tmpdir="$$(mktemp -d)" ;\
		archive="helm-$(HELM_VERSION)-$$(go env GOOS)-$$(go env GOARCH).tar.gz" ;\
		curl -sSLo "$${tmpdir}/$${archive}" "https://get.helm.sh/$${archive}" ;\
		tar -xzf "$${tmpdir}/$${archive}" -C "$${tmpdir}" ;\
		cp "$${tmpdir}/$$(go env GOOS)-$$(go env GOARCH)/helm" "$(HELM)" ;\
		rm -rf "$${tmpdir}" ;\
	fi

.PHONY: pandoc-bin
pandoc-bin: $(PANDOC) ## Download pandoc locally if necessary.
$(PANDOC): $(LOCALBIN)
	@if ! [ -f "$(PANDOC)" ]; then \
		if [ "$(shell go env GOOS)" != "darwin" ]; then \
			curl -sSLo $(PANDOC).tar.gz https://github.com/jgm/pandoc/releases/download/$(PANDOC_VERSION)/pandoc-$(PANDOC_VERSION)-$(shell go env GOOS)-$(shell go env GOARCH).tar.gz ;\
			tar xv --directory=$(LOCALBIN) --file=$(PANDOC).tar.gz pandoc-$(PANDOC_VERSION)/bin/pandoc --strip-components=2 ;\
		else \
			curl -sSLo $(PANDOC).zip https://github.com/jgm/pandoc/releases/download/$(PANDOC_VERSION)/pandoc-$(PANDOC_VERSION)-$(shell go env GOARCH)-macOS.zip ;\
			unzip -oqqjd $(LOCALBIN) $(PANDOC).zip ;\
		fi ;\
		mv $(LOCALBIN)/pandoc $(PANDOC) ;\
		rm -f $(PANDOC).tar.gz $(PANDOC).zip ;\
	fi


.PHONY: operator-sdk
operator-sdk: ## Download operator-sdk locally if necessary.
ifeq (,$(wildcard $(OPERATOR_SDK)))
ifeq (, $(shell which operator-sdk 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPERATOR_SDK)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPERATOR_SDK) https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$${OS}_$${ARCH} ;\
	chmod +x $(OPERATOR_SDK) ;\
	}
else
OPERATOR_SDK = $(shell which operator-sdk)
endif
endif

##@ Development

.PHONY: install-dev
install-dev: ## Install binaries for development environment.
	go install github.com/go-delve/delve/cmd/dlv@latest
	go install sigs.k8s.io/kind@latest
	go install sigs.k8s.io/cloud-provider-kind@latest

.PHONY: helm-validate
helm-validate: helm-dependency-update helm-lint ## Validate Helm charts.

.PHONY: helm-docs
helm-docs: helm-docs-bin ## Run helm-docs.
	$(HELM_DOCS) --chart-search-root=helm

.PHONY: helm-lint
helm-lint: helm-bin ## Lint Helm charts.
	find "helm/" -depth -mindepth 1 -maxdepth 1 -type d -print0 | xargs -0r -n1 $(HELM) lint --strict

.PHONY: helm-unittest
helm-unittest: helm-unittest-bin ## Run helm-unittest.
	find "helm/" -depth -mindepth 1 -maxdepth 1 -type d -print0 | xargs -0r -n1 $(HELM) unittest --strict

.PHONY: helm-unittest-update
helm-unittest-update: helm-unittest-bin ## Update helm-unittest snapshots.
	find "helm/" -depth -mindepth 1 -maxdepth 1 -type d -print0 | xargs -0r -n1 $(HELM) unittest --strict --update-snapshot

.PHONY: helm-unittest-bin
helm-unittest-bin: helm-bin ## Download helm-unittest plugin locally if necessary.
	@mkdir -p "$(HELM_CONFIG_HOME)" "$(HELM_CACHE_HOME)" "$(HELM_DATA_HOME)" "$(HELM_PLUGINS)"
	$(call helm-install-plugin,unittest,https://github.com/helm-unittest/helm-unittest,$(HELM_UNITTEST_VERSION))


.PHONY: helm-dependency-update
helm-dependency-update: helm-bin ## Update Helm chart dependencies.
	find "helm/" -depth -mindepth 1 -maxdepth 1 -type d -print0 | xargs -0r -n1 $(HELM) dependency update

.PHONY: values-dev
values-dev: ## Safely initialize values-dev.yaml files for Helm charts.
	find "helm/" -type f -name "values.yaml" | $(SED) 'p;s/\.yaml/-dev\.yaml/' | xargs -n2 cp $(CP_FLAGS)

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd:generateEmbeddedObjectMeta=true webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd:generateEmbeddedObjectMeta=true webhook paths="./..." output:crd:artifacts:config=helm/slurm-operator-crds/templates

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
	go generate ./...

.PHONY: generate-docs
generate-docs: pandoc-bin
# Use pandoc to generate index.rst from README.md
	$(PANDOC) --quiet README.md -o docs/index.rst

# Add a newline at the base of index.rst
	printf '\n' >> docs/index.rst

# Generate and insert a table of contents for ./docs
	cat ./docs/_static/toc.rst >> docs/index.rst

# In index.rst, find all instances of links to markdown files in the docs dir, and strip the directory prefix from them
	$(SED) -i -E '/<.\/docs\/[A-Za-z]*.md/s/.\/docs\///g' docs/index.rst

# In index.rst, find all instances of links to svgs and strip the directory prefix from them
	$(SED) -i -E '/.\/docs\/.*.svg/s/.\/docs\///g' docs/index.rst

# In index.rst, find all instances of markdown files within a subdirectory of docs, and strip the directory and subdirectory prefix from them
	$(SED) -i -E '/<.\/docs\/[A-Za-z]*\/[A-Za-z]*.md>`/s/.\/docs\///g' docs/index.rst

# In index.rst, replace all links to markdown files with links to HTML files, as will be present when using Myst Parser in Sphinx
	$(SED) -i -E '/[A-Za-z]*.md>`/s/.md>/.html>/g' docs/index.rst

DOCS_IMAGE ?= $(REGISTRY)/sphinx

.PHONY: build-docs
build-docs: ## Build the container image used to develop the docs
	$(CONTAINER_TOOL) build -t $(DOCS_IMAGE) ./docs

.PHONY: run-docs
run-docs: build-docs ## Run the container image for docs development
	$(CONTAINER_TOOL) run --rm --network host -v ./docs:/docs:z $(DOCS_IMAGE) sphinx-autobuild --port 8000 /docs /build/html

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: tidy
tidy: ## Run go mod tidy against code
	go mod tidy

.PHONY: get-u
get-u: ## Run `go get -u`
	go get -u ./...
	$(MAKE) tidy

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: govulncheck
govulncheck: govulncheck-bin ## Run govulncheck
	$(GOVULNCHECK) ./...

# https://github.com/golangci/golangci-lint/blob/main/.pre-commit-hooks.yaml
.PHONY: golangci-lint
golangci-lint: golangci-lint-bin ## Run golangci-lint.
	$(GOLANGCI_LINT) run --fix

# https://github.com/golangci/golangci-lint/blob/main/.pre-commit-hooks.yaml
.PHONY: golangci-lint-fmt
golangci-lint-fmt: golangci-lint-bin ## Run golangci-lint fmt.
	$(GOLANGCI_LINT) fmt

CODECOV_PERCENT ?= 66

.PHONY: test
test: envtest ## Run tests.
	rm -f cover.out cover.html
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
	go test `go list ./... | grep -v "/api" | grep -v "/e2e"` -v -coverprofile cover.out
	go tool cover -func cover.out
	go tool cover -html cover.out -o cover.html
	@percentage=$$(go tool cover -func=cover.out | grep ^total | awk '{print $$3}' | tr -d '%'); \
		if (( $$(echo "$$percentage < $(CODECOV_PERCENT)" | bc -l) )); then \
			echo "----------"; \
			echo "Total test coverage ($${percentage}%) is less than the coverage threshold ($(CODECOV_PERCENT)%)."; \
			exit 1; \
		fi

.PHONY: benchmark
benchmark: envtest ## Run benchmark tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
	go test `go list ./... | grep -v "/api" | grep -v "/e2e"` -bench=. -benchmem

.PHONY: test-e2e
test-e2e:
	go test -v -timeout 30m ./test/e2e
