all: build

.PHONY: mod
mod:
	go mod download

.PHONY: build
build: mod
	CGO_ENABLED=0 go build -o build/proxy ./cmd/

# VERSION=0.0.9 make docker-build
.PHONY: docker-build
docker-build: build
	docker build --tag=unibackend/uniproxy:$(VERSION) .


# VERSION=0.0.9 make docker-push
.PHONY: docker-push
docker-push: docker-build
	docker push unibackend/uniproxy:$(VERSION)