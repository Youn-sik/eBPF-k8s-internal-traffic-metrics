# eBPF 빌드 사전 준비 (Linux, arm64 예시)

Linux 환경에서 eBPF C 코드를 빌드하고 Go 바인딩을 생성하기 위한 준비 절차입니다.

## 1) 필수 패키지 설치
```bash
sudo apt-get update
sudo apt-get install -y wget tar
```

## 2) Go 설치 (arm64 예시)
```bash
wget https://go.dev/dl/go1.25.0.linux-arm64.tar.gz
sudo tar -C /usr/local -xzf go1.25.0.linux-arm64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

## 3) bpf2go 설치 및 확인
```bash
go install github.com/cilium/ebpf/cmd/bpf2go@latest
ls $(go env GOBIN)/bpf2go 2>/dev/null || ls $(go env GOPATH)/bin/bpf2go
```

## 4) 커널/툴체인 의존성 설치
```bash
sudo apt-get install -y libbpf-dev linux-headers-$(uname -r) clang llvm
```

## 5) 빌드 실행 (arm64)
```bash
cd "$(dirname "$0")"
ARCH=arm64 ./gen_bpf.sh
# 출력 예시:
# [1/2] Dumping vmlinux.h from kernel BTF...
# [2/2] Running bpf2go (arch=arm64, pkg=ebpf)...
# Done. Generated tcp_connect_bpfel.go / tcp_connect_bpfeb.go and vmlinux.h under ..
```

> 다른 아키텍처(x86_64 등)에서는 `ARCH=x86`로 실행하고, 필요한 경우 `GOPACKAGE=<패키지명>` 환경변수로 Go 패키지명을 지정할 수 있습니다.
