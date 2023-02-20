define DOCKERFILE_TEST
FROM amd64/$(BASE_IMAGE)
RUN apk add --no-cache make docker-cli git
WORKDIR /s
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
endef
export DOCKERFILE_TEST

test:
	echo "$$DOCKERFILE_TEST" | docker build -q . -f - -t temp
	docker run --rm \
	temp \
	make test-nodocker

test-nodocker:
	$(eval export CGO_ENABLED=0)
	go build -o /dev/null .
