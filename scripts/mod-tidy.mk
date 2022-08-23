mod-tidy:
	docker run --rm -it -v $(PWD):/s -w /s amd64/$(BASE_IMAGE) \
	sh -c "apk add git && go get && GOPROXY=direct go mod tidy"
