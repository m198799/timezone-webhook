.DEFAULT_GOAL := build

# Build Variables
ROOT_PACKAGE=github.com/m198799/timezone-webhook
BINARY_NAME ?= webhook
OUT_DIR ?= ./bin/
VERSION ?= 0.0.7
TARGET=/usr/local/bin
INSTALLCMD=install -v $(OUT_DIR)$(BINARY_NAME) $(TARGET)
BUILD_FLAGS ?= \
	-ldflags="-s -w \
	-X '$(MODULE)pkg/version.GitCommit=$(GIT_COMMIT)' \
	-X '$(MODULE)pkg/version.AppVersion=$(VERSION)' \
	-X '$(MODULE)pkg/version.ImageRepository=$(IMAGE_REPOSITORY)'"

MODULE = github.com/m198799/timezone-webhook

# Docker Image Variables
IMAGE_REPOSITORY ?= registry.jugglechat.cn/timezone-webhook

HELM_REPOSITORY ?= oci://registry.jugglechat.cn/helm

IMAGE ?= $(IMAGE_REPOSITORY):$(VERSION)$(VERSION_SUFFIX)

export HELM_EXPERIMENTAL_OCI=1

# Targets
install: compile
		if [ -w $(TARGET) ]; then \
		$(INSTALLCMD); else \
		sudo $(INSTALLCMD); fi

clean:
		rm -rfv "$(OUT_DIR)"

test:
		go test -v ./...

fmt:
	go fmt ./...

# https://godoc.org/golang.org/x/tools/cmd/goimports
imports:
	bash scripts/goimports_helper.sh
	goimports -e -d -w -local $(ROOT_PACKAGE) ./

# https://github.com/golang/lint
# go get github.com/golang/lint/golint
lint:
	golint ./...

# https://github.com/golangci/golangci-lint/
# Install: go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.34.1
cilint:
	golangci-lint -c ./.golangci.yaml run ./...


coverage-report:
		go test -coverprofile build/coverage-report.html ./...
		go tool cover -html build/coverage-report.html

build: compile # Alias

# 不要随便给他加GOARCH=amd64参数, 因为这个会用于多指令集镜像的构建
compile:
		CGO_ENABLED=0 \
		GOARCH=$(TARGETARCH) \
		go build \
		-v \
		-o $(OUT_DIR)$(BINARY_NAME) \

# 不要随便给他加GOARCH=amd64参数, 因为这个会用于多指令集镜像的构建
goc-build:
		CGO_ENABLED=0 GO111MODULE=on \
		GOARCH=$(TARGETARCH) \
		goc build \
		--center ${GOC_CENTER} \
		--buildflags="-a -ldflags '-w -s'" \
		-o $(OUT_DIR)$(BINARY_NAME)

compile-skaffold:
		CGO_ENABLED=0 \
		GOARCH=$(TARGETARCH) \
		go build \
		-gcflags="${SKAFFOLD_GO_GCFLAGS}" \
		-v \
		-o $(OUT_DIR)$(BINARY_NAME) \

compile-linux:
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build \
		-v \
		-o $(OUT_DIR)$(BINARY_NAME) \

docker: docker-build # Alias

docker-build: compile
		docker build \
		-t $(IMAGE) \
		-f Dockerfile \
		.

docker-build-linux: compile-linux
		docker build \
        		-t $(IMAGE) \
        		-f Dockerfile \
        		.
docker-push: docker-build-linux
		docker push $(IMAGE)


helm: helm-package # Alias

helm-package: helm-lint
		@rm -rfv $(OUT_DIR)webhook-*.tgz
		helm package \
		-d $(OUT_DIR) \
		--app-version $(VERSION) \
		charts/webhook/

helm-lint:
		helm lint charts/webhook/

helm-install: helm-package helm-uninstall
		helm install webhook $(OUT_DIR)webhook-*.tgz

helm-uninstall:
		@helm status webhook 2>&1 > /dev/null && echo Uninstalling helm package... && helm uninstall webhook || true

helm-push:
	cd ./build && helm push webhook-$(VERSION).tgz $(HELM_REPOSITORY)

release: test compile docker helm

# Phony Targets
.PHONY: install clean build test tzdata coverage-report compile docker docker-build docker-push helm-lint helm helm-package helm-install helm-uninstall release

check: test fmt imports cilint