define DOCKERFILE_BINARIES
FROM amd64/$(BASE_IMAGE)
RUN apk add --no-cache zip make git tar
WORKDIR /s
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN make binaries-nodocker
endef
export DOCKERFILE_BINARIES

binaries:
	echo "$$DOCKERFILE_BINARIES" | docker build . -f - -t temp \
	&& docker run --rm -v $(PWD):/out \
	temp sh -c "rm -rf /out/binaries && cp -r /s/binaries /out/"

binaries-nodocker:
	$(eval export CGO_ENABLED=0)
	$(eval VERSION := $(shell git describe --tags))
	$(eval GOBUILD := go build -ldflags '-X main.version=$(VERSION)')
	rm -rf tmp && mkdir tmp
	rm -rf binaries && mkdir binaries

	GOOS=linux GOARCH=amd64 $(GOBUILD) -o tmp/landiscover
	tar -C tmp -czf $(PWD)/binaries/landiscover_$(VERSION)_linux_amd64.tar.gz --owner=0 --group=0 landiscover

	GOOS=linux GOARCH=arm GOARM=6 $(GOBUILD) -o tmp/landiscover
	tar -C tmp -czf $(PWD)/binaries/landiscover_$(VERSION)_linux_arm6.tar.gz --owner=0 --group=0 landiscover

	GOOS=linux GOARCH=arm GOARM=7 $(GOBUILD) -o tmp/landiscover
	tar -C tmp -czf $(PWD)/binaries/landiscover_$(VERSION)_linux_arm7.tar.gz --owner=0 --group=0 landiscover

	GOOS=linux GOARCH=arm64 $(GOBUILD) -o tmp/landiscover
	tar -C tmp -czf $(PWD)/binaries/landiscover_$(VERSION)_linux_arm64v8.tar.gz --owner=0 --group=0 landiscover
