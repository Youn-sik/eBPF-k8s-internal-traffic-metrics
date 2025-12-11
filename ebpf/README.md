# eBPF 빌드/바인딩 생성 가이드

이 디렉터리는 커널용 eBPF 코드(`tcp_connect.c`)와 Go 바인딩을 생성하기 위한 스크립트를 포함합니다.

## 요구 사항
- Linux 커널에서 BTF 사용 가능(`/sys/kernel/btf/vmlinux` 존재)
- `bpftool` (v5.8+ 권장)
- `clang/llvm`
- Go 1.20+ 및 `bpf2go` (`go install github.com/cilium/ebpf/cmd/bpf2go@latest`)

## 생성 방법
```bash
# x86_64 노드 기준 (리틀 엔디안)
cd ebpf
ARCH=x86 ./gen_bpf.sh

# arm64 노드 기준
ARCH=arm64 ./gen_bpf.sh
```

생성 결과:
- `vmlinux.h` : 대상 커널 BTF로부터 생성
- `tcp_connect_bpfel.go` : 리틀 엔디안용 Go 바인딩
- `tcp_connect_bpfeb.go` : 빅 엔디안용 Go 바인딩 (포터빌리티용)

> macOS 등 비-Linux 환경에서는 스크립트 실행이 실패합니다. 실제 대상 클러스터 노드(또는 같은 커널 버전의 Linux VM/컨테이너)에서 실행하세요.

### 패키지명 제어
- 기본 Go 패키지명은 `ebpf`입니다.
- 변경하려면 `GOPACKAGE=<원하는패키지>` 환경변수로 실행하세요.
  ```bash
  GOPACKAGE=agentebpf ARCH=arm64 ./gen_bpf.sh
  ```
