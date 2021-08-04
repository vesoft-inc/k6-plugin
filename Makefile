all: build
.PHONY: build

pairs := darwin/amd64 linux/amd64 linux/arm64
GOPATH := $${GOPATH:-~/go}
export GO111MODULE=on

build: 
	go install github.com/k6io/xk6/cmd/xk6@latest
	$(GOPATH)/bin/xk6 build --with github.com/HarrisChu/xk6-nebula@latest; 

build-all: build-arm-v7

	go install github.com/k6io/xk6/cmd/xk6@latest
	for  pair in $(pairs);do echo $$pair; \
			os=`echo $$pair | cut -d / -f 1 ` ;\
			arch=`echo $$pair | cut -d / -f 2 ` ;\
			GOOS=$$os GOARCH=$$arch  $(GOPATH)/bin/xk6 build --with github.com/HarrisChu/xk6-nebula@latest ;\
			mv k6 k6-$$os-$$arch; \
	done

build-arm-v7:
	go install github.com/k6io/xk6/cmd/xk6@latest
	GOOS=linux GOARCH=arm64 GOARM=7  $(GOPATH)/bin/xk6 build --with github.com/HarrisChu/xk6-nebula@latest;	
	mv k6 k6-linux-arm64-v7

build-dev:
	go install github.com/k6io/xk6/cmd/xk6@latest
	$(GOPATH)/bin/xk6 build --with github.com/HarrisChu/xk6-nebula@latest=${PWD}/../xk6-nebula; 	

.PHONY: format
format:
	find . -name '*.go' -exec gofmt -s -w {} +
