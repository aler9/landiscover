define DOCKERFILE_RUN
FROM amd64/$(BASE_IMAGE)
WORKDIR /s
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN go build -o /out .
endef
export DOCKERFILE_RUN

run:
	echo "$$DOCKERFILE_RUN" | docker build -q . -f - -t temp
	docker run --rm -it \
	--network=host \
	temp /out $(ARGS)
