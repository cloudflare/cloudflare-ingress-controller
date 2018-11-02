#PROJECT = cloudflare-ingress-controller
PROJECT = argo-tunnel
REGISTRY ?= gcr.io/cloudflare-registry
IMAGE := $(REGISTRY)/$(PROJECT)

PLATFORMS := \
	amd64-linux-amd64 \
	arm64v8-linux-arm64 \
	ppc64le-linux-ppc64le
	# ARM, consider supporting 32bit
	# arm32v6-linux-arm-5
	# arm32v6-linux-arm-6
	# arm32v6-linux-arm-7
	# x86, consider supporting 32bit
	# i386-linux-386
	# QEMU issue, exec failure on apk add
	# s390x-linux-s390x

COMMA := ,
CONTAINERS := $(addprefix container-, $(PLATFORMS))
CONTAINER_ARGS = s/ARG_VERSION/$(1)/g;s/ARG_ROOT/$(2)/g;s/ARG_OS/$(3)/g;s/ARG_ARCH/$(4)/g;s/ARG_ARM/$(5)/g
PUSHES := $(addprefix push-, $(PLATFORMS))
JOBS := $(addprefix job-, $(PLATFORMS))
MANIFESTS =

SRCS := $(shell go list ./cmd/... ./internal/...)
SRC_DIRS := ./cmd ./internal
TMP_DIR := .build

VERSION ?= $(shell git describe --tags --always --dirty)

.PHONY: default
default: clean;

.PHONY: build-dir
build-dir:
	@echo generating build directory
	@mkdir -p $(TMP_DIR)

.PHONY: check
check: test-race vet fmt staticcheck unused misspell

.PHONY: clean
clean:
	@echo cleaning build targets
	@rm -rf $(TMP_DIR) bin coverage.txt

.PHONY: container
container:
	@echo build docker container
	@docker build --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) .

.PHONY: cross $(CONTAINERS) $(PUSHES) $(JOBS)
cross: $(JOBS)
	@echo building multi-arch manifest
	@docker manifest create --amend $(IMAGE):$(VERSION) $(MANIFESTS)
	@docker manifest push $(IMAGE):$(VERSION)

$(CONTAINERS): container-%: build-dir
	@echo build docker container $*
	$(eval $@_ARGS := $$(call CONTAINER_ARGS,$(VERSION),$(subst -,$(COMMA),$*)))
	$(eval $@_ARCH := $$(firstword $(subst -, ,$*)))
	@sed -e "$($@_ARGS)" hack/Dockerfile > $(TMP_DIR)/Dockerfile.$*
	@docker build -t $(IMAGE)-$($@_ARCH):$(VERSION) -f $(TMP_DIR)/Dockerfile.$* .

$(PUSHES): push-%: container-%
	@echo push docker container $*
	$(eval $@_ARCH := $$(firstword $(subst -, ,$*)))
	@docker push $(IMAGE)-$($@_ARCH):$(VERSION)

$(JOBS): job-%: push-%
	$(eval $@_ARCH := $$(firstword $(subst -, ,$*)))
	$(eval MANIFESTS += $(IMAGE)-$($@_ARCH):$(VERSION))

.PHONY: dep
dep:
	@echo ensure dependencies
	@dep ensure -vendor-only -v

.PHONY: fmt
fmt:  
	@echo checking code is formatted
	@test -z "$(shell gofmt -s -l -d -e $(SRC_DIRS) | tee /dev/stderr)"

.PHONY: helm
helm:
	@echo generating helm manifest
	@helm template --name=$(VERSION) chart/

.PHONY: install
install:
	@echo installing build targets
	@go install -v ./...

.PHONY: lint
lint:
	@echo checking code is linted
	@go get golang.org/x/lint/golint
	@golint $(shell go list ./... | grep -v /vendor/)

.PHONY: misspell
misspell:
	@echo checking for misspellings
	@go get github.com/client9/misspell/cmd/misspell
	@misspell \
		-i clas \
		-locale US \
		-error \
		cmd/* internal/* docs/* *.md

.PHONY: push
push: container
	@docker push $(IMAGE):$(VERSION)
	@if git describe --tags --exact-match >/dev/null 2>&1; \
	then \
		docker tag $(IMAGE):$(VERSION) $(IMAGE):latest; \
		docker push $(IMAGE):latest; \
	fi

.PHONY: staticcheck
staticcheck:
	@echo static checking code for issues
	@go get honnef.co/go/tools/cmd/staticcheck
	@staticcheck $(SRCS)

.PHONY: test
test: install
	@echo testing code for issues
	@go test -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: test-race
test-race: | test
	@echo testing code for races
	@go test -coverprofile=coverage.txt -covermode=atomic -race ./...

.PHONY: unused
unused:
	@echo checking code for unused definitions
	@go get honnef.co/go/tools/cmd/unused
	@unused -exported $(SRCS)

.PHONY: version
version:
	@echo $(VERSION)

.PHONY: vet
vet: | test
	@echo checking code is vetted
	@go vet $(shell go list ./... | grep -v /vendor/)
