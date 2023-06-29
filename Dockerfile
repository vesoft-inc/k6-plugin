FROM golang:1.20.5 as builder

COPY . /app/k6-plugin

RUN cd /app/k6-plugin \
&& make build-dev

FROM centos:7 

RUN  yum install -y --nogpgcheck\
		make \
		git 

COPY --from=builder /app/k6-plugin/k6 /app/k6

WORKDIR /app
	