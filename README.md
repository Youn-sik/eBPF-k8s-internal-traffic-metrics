## internal-tcp-watcher: 내부 트래픽 감지 파이프라인

**목표:** 파드가 0개일 때도 내부 TCP connect 시도를 놓치지 않고 포착해 KEDA로 스케일 신호를 보내는 초경량 에이전트.

---

## 전체 흐름 (L4 TCP 기준)

1) **사전 준비 — Mapping (Go)**
   - K8s API(Services/EndpointSlices)를 감시해 ClusterIP/PodIP를 매핑 테이블에 저장.
2) **연결 포착 — Interception (C/eBPF)**
   - `kprobe/tcp_v4_connect`에서 목적지 IP와 comm(프로세스명)을 ringbuf로 전달.
3) **메트릭 변환 — Translation (Go)**
   - ringbuf 이벤트를 매핑 테이블로 식별해 Prometheus 카운터 증가:
     ```
     internal_tcp_attempts_total{
       destination_namespace,
       destination_service,
       destination_pod,
       process_comm
     } += 1
     ```
   - 기본 필터: `EXCLUDE_COMMS=kubelet`로 kubelet 헬스체크는 카운트/로그에서 제외(필터 로그는 남김).
4) **스케일 트리거 — Triggering (VM Agent + KEDA)**
   - VM Agent가 `/metrics`를 스크레이프.
   - KEDA가 `sum(rate(internal_tcp_attempts_total{...}[1m]))` 등으로 증가율을 감지해 `0 -> 1` 스케일.

---

## 왜 0 파드에서도 감지 가능한가?
- 호출이 ClusterIP로 들어오면, 파드가 없더라도 커널의 `tcp_v4_connect` 진입부에서 목적지 IP를 포착한다.
- eBPF가 ringbuf로 이벤트를 올리고, 매핑 테이블이 Service/ClusterIP를 알고 있으므로 시계열은 살아 있다.
- 파드가 0이어도 메트릭 시계열은 유지되고, rate/increase로 “최근 증가”만 트리거에 사용한다.

### 한계
- Headless/Pod IP 직접 호출: 파드가 0이면 IP가 없으므로 감지 불가(ClusterIP 경로로 유도 필요).
- 외부 트래픽: ingress/L7 레이어에서 별도 메트릭 사용(node-exporter/kube-state-metrics/ingress controller).

---

## 기술 스택과 역할

| 구성 요소 | 기술 스택 | 핵심 역할 |
| --- | --- | --- |
| 커널 감시자 | C + eBPF | `tcp_v4_connect`에서 목적지 IP+comm 캡처 |
| 통역사 | Go (User Space) | IP→Service 매핑, 필터 적용, /metrics 노출 |
| 저장소 | VictoriaMetrics/Prometheus | 메트릭 수집·rate 계산 |
| 실행자 | KEDA | 메트릭 증가율을 보고 파드 기동 |

---

## 메트릭 사용법
- 기본 주소: `METRICS_ADDR` (기본 `0.0.0.0:9102`, hostNetwork DaemonSet).
- 카운터: `internal_tcp_attempts_total{destination_namespace, destination_service, destination_pod, process_comm}`.
- 필터: `EXCLUDE_COMMS`(기본 kubelet) 환경변수로 comm 기반 제외.
- 확인 예: `curl -s http://<nodeIP>:9102/metrics | grep internal_tcp_attempts_total`.
- KEDA 예시: `sum(rate(internal_tcp_attempts_total{destination_namespace="default",destination_service="my-svc"}[1m]))`.

---

## 배포/테스트
- DaemonSet: `ebpf/kubernetes/daemonset.yaml` (hostNetwork, METRICS_ADDR=0.0.0.0:9102).
- 샘플 트래픽: `ebpf/kubernetes/sample-tcp-client.yaml` (ClusterIP 172.20.230.242:8429로 nc -z 반복).

---

## 운영 시 유의사항
- Counter는 float64로 오버플로우 걱정은 사실상 없음. 시계열 폭증을 막으려면 라벨 조합을 관리하고 필요 시 relabel_drop 고려.
- comm 필터는 기본 kubelet 제외. 추가 제외는 `EXCLUDE_COMMS`로 설정.
- 새 eBPF 필드 추가 시 `gen_bpf.sh` 실행 후 `artifacts/<arch>/` 산출물을 갱신해야 한다.

이 구성을 통해 **L4 eBPF 에이전트 → VM/Prometheus → KEDA**로 이어지는 내부 트래픽 감지 파이프라인을 완성합니다.
