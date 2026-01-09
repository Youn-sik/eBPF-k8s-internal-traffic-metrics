## k8s-ebpf-l4l7-metrics: eBPF 기반 내부 트래픽 감지 에이전트

**목표:** 파드가 0개일 때도 내부 TCP/HTTP 트래픽을 감지하여 KEDA 스케일 신호를 제공하는 초경량 eBPF 에이전트.

---

## 주요 기능

| 계층 | 감지 대상 | Hook Point | 메트릭 |
|------|----------|------------|--------|
| **L4** | TCP 송신 연결 시도 | `kprobe/tcp_v4_connect` | `internal_tcp_attempts_total` |
| **L7** | HTTP 수신 요청 | `tracepoint/syscalls/sys_*_read` | `internal_http_requests_total` |

---

## 전체 흐름

### L4 TCP 송신 감지
1) **K8s Mapping** — Services/EndpointSlices 감시하여 ClusterIP/PodIP 매핑 테이블 유지
2) **eBPF Interception** — `tcp_v4_connect`에서 목적지 IP와 프로세스명(comm) 캡처
3) **Metric Translation** — IP를 K8s 메타데이터로 변환하여 Prometheus 카운터 증가
4) **KEDA Trigger** — `sum(rate(internal_tcp_attempts_total{...}[1m]))` 기반 스케일링

### L7 HTTP 수신 감지
1) **Accept Tracking** — `sys_enter/exit_accept4`에서 클라이언트 IP/Port 캡처
2) **Read Interception** — `sys_enter/exit_read`에서 HTTP 요청 메서드/경로 파싱
3) **Health Check Filter** — `/healthz`, `/ready` 등 헬스체크 경로 자동 제외
4) **Metric Export** — 메서드, 경로, 네임스페이스별 카운터 증가

---

## 왜 0 파드에서도 감지 가능한가?

- 호출이 ClusterIP로 들어오면, 파드가 없더라도 커널의 `tcp_v4_connect` 진입부에서 목적지 IP를 포착
- eBPF가 ringbuf로 이벤트를 전송하고, 매핑 테이블이 Service/ClusterIP를 알고 있으므로 시계열 유지
- 파드가 0이어도 메트릭 시계열은 살아있고, rate/increase로 "최근 증가"만 트리거에 사용

### 한계
- Headless/Pod IP 직접 호출: 파드가 0이면 IP가 없으므로 감지 불가 (ClusterIP 경로로 유도 필요)
- HTTPS 트래픽: TLS 암호화로 페이로드 파싱 불가 (L4 메트릭만 수집)
- HTTP/2, gRPC: 바이너리 프로토콜로 별도 파서 필요 (향후 확장)

---

## 메트릭 스키마

### L4 메트릭 (송신)
```text
internal_tcp_attempts_total{
  destination_namespace,
  destination_service,
  destination_pod,
  process_comm
}
```

### L7 메트릭 (수신)
```text
internal_http_requests_total{
  source_ip,
  destination_namespace,
  destination_service,
  destination_pod,
  method,
  path,
  process_comm
}
```

---

## 환경변수 설정

| 환경변수 | 기본값 | 설명 |
|---------|-------|------|
| `MODE` | `cluster` | 실행 모드 (cluster/local) |
| `METRICS_ADDR` | `0.0.0.0:9102` | 메트릭 서버 주소 |
| `WATCH_NAMESPACE` | (전체) | 감시 네임스페이스 |
| `MAPPER_TTL` | `60s` | 매핑 캐시 TTL |
| `MAPPER_CAPACITY` | `2048` | 매핑 테이블 최대 엔트리 |
| `EXCLUDE_COMMS` | `kubelet` | L4 제외 프로세스 (쉼표 구분) |
| `ENABLE_L4` | `true` | L4 TCP 감지 활성화 |
| `ENABLE_L7_HTTP` | `false` | L7 HTTP 감지 활성화 |
| `FILTER_HEALTHCHECK` | `true` | 헬스체크 경로 필터링 |
| `HEALTHCHECK_PATHS` | (내장 패턴) | 추가 헬스체크 경로 |

---

## 배포

### 빌드
```bash
cd ebpf
make build PUSH=true VERSION=0.1.0
```

### 배포
```bash
make deploy NAMESPACE=skuber-system
```

### 메트릭 확인
```bash
curl -s http://<nodeIP>:9102/metrics | grep internal_
```

---

## KEDA 예시

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
spec:
  triggers:
    - type: prometheus
      metadata:
        query: sum(rate(internal_tcp_attempts_total{destination_service="my-svc"}[1m]))
        threshold: "1"
```

---

## 기술 스택

| 구성 요소 | 기술 | 역할 |
|----------|------|------|
| 커널 감시자 | C + eBPF | TCP connect / HTTP read 캡처 |
| 유저스페이스 | Go | IP→Service 매핑, 메트릭 노출 |
| 저장소 | VictoriaMetrics/Prometheus | 메트릭 수집·쿼리 |
| 스케일러 | KEDA | 메트릭 기반 오토스케일링 |

---

## 라이선스

Dual BSD/GPL
