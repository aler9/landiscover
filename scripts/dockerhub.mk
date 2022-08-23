define DOCKERFILE_DOCKERHUB
FROM --platform=linux/amd64 $(BASE_IMAGE) AS build
RUN apk add --no-cache git
WORKDIR /s
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
ARG VERSION
ARG OPTS
RUN export CGO_ENABLED=0 $${OPTS} \
	&& go build -ldflags "-X main.version=$$VERSION" -o /landiscover

FROM scratch
COPY --from=build /landiscover /landiscover
ENTRYPOINT [ "/landiscover" ]
endef
export DOCKERFILE_DOCKERHUB

dockerhub:
	$(eval export DOCKER_CLI_EXPERIMENTAL=enabled)
	$(eval VERSION := $(shell git describe --tags))

	docker buildx rm builder 2>/dev/null || true
	rm -rf $$HOME/.docker/manifests/*
	docker buildx create --name=builder --use

	echo "$$DOCKERFILE_DOCKERHUB" | docker buildx build . -f - --build-arg VERSION=$(VERSION) \
	--push -t aler9/landiscover:$(VERSION)-amd64 --build-arg OPTS="GOOS=linux GOARCH=amd64" --platform=linux/amd64

	echo "$$DOCKERFILE_DOCKERHUB" | docker buildx build . -f - --build-arg VERSION=$(VERSION) \
	--push -t aler9/landiscover:$(VERSION)-armv6 --build-arg OPTS="GOOS=linux GOARCH=arm GOARM=6" --platform=linux/arm/v6

	echo "$$DOCKERFILE_DOCKERHUB" | docker buildx build . -f - --build-arg VERSION=$(VERSION) \
	--push -t aler9/landiscover:$(VERSION)-armv7 --build-arg OPTS="GOOS=linux GOARCH=arm GOARM=7" --platform=linux/arm/v7

	echo "$$DOCKERFILE_DOCKERHUB" | docker buildx build . -f - --build-arg VERSION=$(VERSION) \
	--push -t aler9/landiscover:$(VERSION)-arm64v8 --build-arg OPTS="GOOS=linux GOARCH=arm64" --platform=linux/arm64/v8

	docker manifest create aler9/landiscover:$(VERSION) \
	$(foreach ARCH,amd64 armv6 armv7 arm64v8,aler9/landiscover:$(VERSION)-$(ARCH))
	docker manifest push aler9/landiscover:$(VERSION)

	docker manifest create aler9/landiscover:latest-amd64 aler9/landiscover:$(VERSION)-amd64
	docker manifest push aler9/landiscover:latest-amd64

	docker manifest create aler9/landiscover:latest-armv6 aler9/landiscover:$(VERSION)-armv6
	docker manifest push aler9/landiscover:latest-armv6

	docker manifest create aler9/landiscover:latest-armv7 aler9/landiscover:$(VERSION)-armv7
	docker manifest push aler9/landiscover:latest-armv7

	docker manifest create aler9/landiscover:latest-arm64v8 aler9/landiscover:$(VERSION)-arm64v8
	docker manifest push aler9/landiscover:latest-arm64v8

	docker manifest create aler9/landiscover:latest \
	$(foreach ARCH,amd64 armv6 armv7 arm64v8,aler9/landiscover:$(VERSION)-$(ARCH))
	docker manifest push aler9/landiscover:latest

	docker buildx rm builder
	rm -rf $$HOME/.docker/manifests/*
