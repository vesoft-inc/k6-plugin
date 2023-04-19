all: build
.PHONY: build

pairs := darwin/amd64 linux/amd64 linux/arm64
GOPATH ?= ~/go
export GO111MODULE=on
VERSION ?= v1.0.1
K6_VERSION ?= v0.43.0

fmt:
	find . -name '*.go' -exec gofmt -s -w {} +

lint :
	golangci-lint run --out-format=tab ./...

build: 
	go install github.com/k6io/xk6/cmd/xk6@v0.4.1
	$(GOPATH)/bin/xk6 build $(K6_VERSION) --with github.com/vesoft-inc/k6-plugin@$(VERSION); 

build-all: build-arm-v7

	go install github.com/k6io/xk6/cmd/xk6@v0.4.1
	for  pair in $(pairs);do echo $$pair; \
		os=`echo $$pair | cut -d / -f 1 ` ;\
		arch=`echo $$pair | cut -d / -f 2 ` ;\
		GOOS=$$os GOARCH=$$arch $(GOPATH)/bin/xk6 build $(K6_VERSION) --with github.com/vesoft-inc/k6-plugin@$(VERSION) ;\
		mv k6 k6-$$os-$$arch; \
	done
	cd tools
	for  pair in $(pairs);do echo $$pair; \
		os=`echo $$pair | cut -d / -f 1 ` ;\
		arch=`echo $$pair | cut -d / -f 2 ` ;\
		GOOS=$$os GOARCH=$$arch go build ;\
		mv tools tools-$$os-$$arch-; \
	done

build-arm-v7:
	go install github.com/k6io/xk6/cmd/xk6@v0.4.1
	GOOS=linux GOARCH=arm64 GOARM=7 $(GOPATH)/bin/xk6 build $(K6_VERSION) --with github.com/vesoft-inc/k6-plugin@$(VERSION);	
	mv k6 k6-linux-arm64-v7

build-dev:
	go install github.com/k6io/xk6/cmd/xk6@v0.4.1
	$(GOPATH)/bin/xk6 build $(K6_VERSION) --with github.com/vesoft-inc/k6-plugin@latest=${PWD}/../k6-plugin; 	

build-dev-all:
	go install github.com/k6io/xk6/cmd/xk6@v0.4.1
	for  pair in $(pairs);do echo $$pair; \
			os=`echo $$pair | cut -d / -f 1 ` ;\
			arch=`echo $$pair | cut -d / -f 2 ` ;\
			GOOS=$$os GOARCH=$$arch $(GOPATH)/bin/xk6 build $(K6_VERSION) --with github.com/vesoft-inc/k6-plugin@latest=${PWD}/../k6-plugin;\
			mv k6 k6-$$os-$$arch; \
	done	

.PHONY: format
format:
	find . -name '*.go' -exec gofmt -s -w {} +
