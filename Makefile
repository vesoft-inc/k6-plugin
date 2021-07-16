all: build

pairs := darwin/amd64 linux/amd64 linux/arm64

.PHONY: build
build:
	go install github.com/k6io/xk6/cmd/xk6@latest
	xk6 build --with github.com/HarrisChu/xk6-nebula@latest; 

build-all: build-arm-v7
	go install github.com/k6io/xk6/cmd/xk6@latest
	for  pair in $(pairs);do echo $$pair; \
			os=`echo $$pair | cut -d / -f 1 ` ;\
			arch=`echo $$pair | cut -d / -f 2 ` ;\
			echo $$os; echo $$arch ;\
			GOOS=$$var_os GOARCH=$$var_arch  xk6 build --with github.com/HarrisChu/xk6-nebula@latest; \
			mv k6 k6-$$os-$$arch; \
	done

build-arm-v7:
	go install github.com/k6io/xk6/cmd/xk6@latest
	GOOS=linux GOARCH=arm64 GOARM=7  xk6 build --with github.com/HarrisChu/xk6-nebula@latest;	
	mv k6 k6-linux-arm64-v7


.PHONY: format
format:
	find . -name '*.go' -exec gofmt -s -w {} +
