# Build agent binary
ARG GO_VERSION=1.25.1
FROM golang:${GO_VERSION}-alpine AS builder

# buildx가 주입하는 타깃 아키텍처/OS 사용
ARG TARGETARCH
ARG TARGETOS
# 필요 시 수동으로 덮어쓸 수 있도록 ARTIFACT_ARCH 추가(기본은 TARGETARCH)
ARG ARTIFACT_ARCH
ENV CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} GO111MODULE=on

WORKDIR /src

# 아키텍처별로 미리 생성한 bpf2go 산출물/오브젝트를 복사
COPY artifacts/${ARTIFACT_ARCH:-${TARGETARCH}}/tcpconnect_bpfel.go ./tcpconnect_bpfel.go
COPY artifacts/${ARTIFACT_ARCH:-${TARGETARCH}}/tcpconnect_bpfeb.go ./tcpconnect_bpfeb.go
COPY artifacts/${ARTIFACT_ARCH:-${TARGETARCH}}/tcpconnect_bpfel.o ./tcpconnect_bpfel.o
COPY artifacts/${ARTIFACT_ARCH:-${TARGETARCH}}/tcpconnect_bpfeb.o ./tcpconnect_bpfeb.o
COPY artifacts/${ARTIFACT_ARCH:-${TARGETARCH}}/vmlinux.h ./vmlinux.h

# 나머지 소스
COPY . .
RUN go mod download
RUN go build -o /out/agent ./cmd/agent

# Minimal runtime image
FROM scratch
WORKDIR /
COPY --from=builder /out/agent /agent
ENTRYPOINT ["/agent"]
