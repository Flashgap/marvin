GINKGO_VERSION ?= v2.27.1
ATLAS_DEV_URL  ?= docker://postgres/16/dev?search_path=public
MIGRATIONS_DIR ?= internal/migrations

.PHONY: build test mockgen migrate-diff migrate-hash

build:
	go build ./...

test:
	go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)
	ginkgo -r --cover --coverprofile=coverprofile.out ./...

mockgen:
	go install go.uber.org/mock/mockgen@v0.6.0
	go generate ./...

# Generate the next migration version for a given driver, e.g.
#   make migrate-diff driver=postgres name=add_lock_streak
# Requires the Atlas CLI: https://atlasgo.io/getting-started
migrate-diff:
	@if [ -z "$(driver)" ] || [ -z "$(name)" ]; then echo "usage: make migrate-diff driver=<postgres|mysql> name=<short_description>"; exit 1; fi
	atlas migrate diff $(name) \
	    --dir "file://$(MIGRATIONS_DIR)/$(driver)" \
	    --dev-url "$(ATLAS_DEV_URL)"
	go run ./internal/migrations/gen_sum

# Recompute atlas.sum files for every driver subdirectory.
migrate-hash:
	go run ./internal/migrations/gen_sum
