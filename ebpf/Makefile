PROJECT ?= library/k8s-ebpf-l4l7-metrics
VERSION ?= 0.0.01
REGISTRY ?= registry.skuberplus.com
IMAGE ?= $(REGISTRY)/$(PROJECT):$(VERSION)
GO_VERSION ?= 1.25.1
# buildx multi-arch settings
PLATFORMS ?= linux/amd64,linux/arm64
PUSH ?= false
NAMESPACE ?= skuber-system

.PHONY: help
help:
	@echo "Targets:"
	@echo "  make build      # docker image build"
	@echo "  make push       # docker push"
	@echo "  make deploy     # kubectl apply daemonset (envsubst)"
	@echo "Vars: PROJECT, VERSION, REGISTRY, IMAGE, GO_VERSION, PLATFORMS, PUSH, NAMESPACE"
	@echo "Note: artifacts/<arch>/ 에 bpf2go 산출물(.go/.o, vmlinux.h)이 있어야 빌드됨"

.PHONY: build
build:
	OUT_OPT=$$( if [ "$(PUSH)" = "true" ]; then echo "--push"; else echo "--load"; fi ); \
	docker buildx build --no-cache --platform=$(PLATFORMS) \
		--build-arg GO_VERSION=$(GO_VERSION) \
		-t $(IMAGE) $$OUT_OPT -f Dockerfile .

.PHONY: push
push:
	docker push $(IMAGE)

.PHONY: deploy
deploy:
	IMAGE=$(IMAGE) NAMESPACE=$(NAMESPACE) ./kubernetes/deploy.sh

.PHONY: clean
clean:
	rm -f bin/agent
