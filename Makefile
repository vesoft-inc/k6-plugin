all: build
.PHONY: build

pairs := darwin/amd64 linux/amd64 linux/arm64
GOPATH ?= ~/go
export GO111MODULE=on
VERSION ?= v0.0.9

build: 
	go install github.com/k6io/xk6/cmd/xk6@v0.4.1
	$(GOPATH)/bin/xk6 build --with github.com/vesoft-inc/k6-plugin@$(VERSION); 

build-all: build-arm-v7

	go install github.com/k6io/xk6/cmd/xk6@v0.4.1
	for  pair in $(pairs);do echo $$pair; \
			os=`echo $$pair | cut -d / -f 1 ` ;\
			arch=`echo $$pair | cut -d / -f 2 ` ;\
			GOOS=$$os GOARCH=$$arch  $(GOPATH)/bin/xk6 build --with github.com/vesoft-inc/k6-plugin@$(VERSION) ;\
			mv k6 k6-$$os-$$arch; \
	done

build-arm-v7:
	go install github.com/k6io/xk6/cmd/xk6@v0.4.1
	GOOS=linux GOARCH=arm64 GOARM=7  $(GOPATH)/bin/xk6 build --with github.com/vesoft-inc/k6-plugin@$(VERSION);	
	mv k6 k6-linux-arm64-v7

build-dev:
	go install github.com/k6io/xk6/cmd/xk6@v0.4.1
	$(GOPATH)/bin/xk6 build --with github.com/vesoft-inc/k6-plugin@latest=${PWD}/../k6-plugin; 	

.PHONY: format
format:
	find . -name '*.go' -exec gofmt -s -w {} +
