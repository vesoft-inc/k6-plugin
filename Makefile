all: build
.PHONY: build

pairs := darwin/amd64 linux/amd64 linux/arm64
GOPATH ?= ~/go
export GO111MODULE=on
K6_VERSION ?= v0.43.0
XK6_VERSION ?= v0.9.2

fmt:
	find . -name '*.go' -exec gofmt -s -w {} +

lint :
	golangci-lint run --out-format=tab ./...

build: 
	go install go.k6.io/xk6/cmd/xk6@${XK6_VERSION}
	version=$(git describe --tags `git rev-list --tags --max-count=1`) \
	$(GOPATH)/bin/xk6 build $(K6_VERSION) --with github.com/vesoft-inc/k6-plugin@$(version) ; 

build-all: build-arm-v7

	go install go.k6.io/xk6/cmd/xk6@${XK6_VERSION}
	for  pair in $(pairs);do echo $$pair; \
		os=`echo $$pair | cut -d / -f 1 ` ;\
		arch=`echo $$pair | cut -d / -f 2 ` ;\
		GOOS=$$os GOARCH=$$arch $(GOPATH)/bin/xk6 build $(K6_VERSION) --with github.com/vesoft-inc/k6-plugin@$(VERSION) ;\
		mv k6 k6-$$os-$$arch; \
	done
	cd tools ;\
	for  pair in $(pairs);do echo $$pair; \
		os=`echo $$pair | cut -d / -f 1 ` ;\
		arch=`echo $$pair | cut -d / -f 2 ` ;\
		GOOS=$$os GOARCH=$$arch go build ;\
		mv tools tools-$$os-$$arch; \
	done

build-arm-v7:
	go install go.k6.io/xk6/cmd/xk6@${XK6_VERSION}
	GOOS=linux GOARCH=arm64 GOARM=7 $(GOPATH)/bin/xk6 build $(K6_VERSION) --with github.com/vesoft-inc/k6-plugin@$(VERSION);	
	mv k6 k6-linux-arm64-v7

build-dev:
	go install go.k6.io/xk6/cmd/xk6@${XK6_VERSION}
	$(GOPATH)/bin/xk6 build $(K6_VERSION) --with github.com/vesoft-inc/k6-plugin@latest=${PWD}/../k6-plugin \
		--replace github.com/vesoft-inc/nebula-go/v3=/home/Harris.chu/code/nebula-go; 	

build-dev-all:
	go install go.k6.io/xk6/cmd/xk6@${XK6_VERSION}
	for  pair in $(pairs);do echo $$pair; \
			os=`echo $$pair | cut -d / -f 1 ` ;\
			arch=`echo $$pair | cut -d / -f 2 ` ;\
			GOOS=$$os GOARCH=$$arch $(GOPATH)/bin/xk6 build $(K6_VERSION) --with github.com/vesoft-inc/k6-plugin@latest=${PWD}/../k6-plugin;\
			mv k6 k6-$$os-$$arch; \
	done	

.PHONY: format
format:
	find . -name '*.go' -exec gofmt -s -w {} +
