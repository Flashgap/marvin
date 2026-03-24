GINKGO_VERSION ?= v2.27.1

.PHONY: build test mockgen

build:
	go build ./...

test:
	go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)
	ginkgo -r --cover --coverprofile=coverprofile.out ./...

mockgen:
	go install go.uber.org/mock/mockgen@v0.6.0
	go generate ./...
