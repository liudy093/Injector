#编译镜像
FROM golang:1.20 as builder
ENV GOPROXY https://goproxy.io
ENV GO111MODULE on
WORKDIR /usr/local/go/src/Injector
COPY ./go.mod .
COPY ./go.sum .
RUN go mod download
COPY . .
#go构建可执行文件,-o 生成Server，放在当前目录
RUN go build -ldflags="-w -s" -o injector .

#执行镜像
FROM ubuntu:latest
WORKDIR /usr/local/go/src/Injector

RUN sed -i 's/deb.debian.org/mirrors.ustc.edu.cn/g' /etc/apt/sources.list && \
    apt update && \
    apt-get install -y curl


RUN cd /tmp &&\
    curl -sLO https://ghproxy.com/https://github.com/argoproj/argo-workflows/releases/download/v3.1.1/argo-linux-amd64.gz &&\
    gunzip argo-linux-amd64.gz &&\
    chmod +x argo-linux-amd64 &&\
    mv ./argo-linux-amd64 /bin/argo

COPY --from=builder /usr/local/go/src/Injector .

ENTRYPOINT ["./injector"]
