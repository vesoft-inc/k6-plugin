all: build

ARCH := amd64
OS = darwin linux

.PHONY: build
build:
	go install github.com/k6io/xk6/cmd/xk6@latest
	xk6 build --with github.com/HarrisChu/xk6-nebula@latest; 

build-all:
	go install github.com/k6io/xk6/cmd/xk6@latest
	for var in $(OS);do echo $$var; \
	GOOS=$$var GOARCH=$(ARCH)  xk6 build --with github.com/HarrisChu/xk6-nebula@latest; \
	mv k6 k6-$$var-$(ARCH); \
	done

.PHONY: format
format:
	find . -name '*.go' -exec gofmt -s -w {} +
