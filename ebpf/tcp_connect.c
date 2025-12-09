// eBPF program: capture destination IPv4 address on tcp_v4_connect and publish via ring buffer.
// NOTE: Requires vmlinux.h generated for target kernel (bpf2go -target bpfel/bpfeb).

#include "vmlinux.h"
#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

struct event {
    __u32 daddr; // Destination IPv4 (network byte order) -> convert in Go using binary.BigEndian
};

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} events SEC(".maps");

SEC("kprobe/tcp_v4_connect")
int BPF_KPROBE(tcp_v4_connect_enter, struct sock *sk)
{
    __u32 daddr;
    struct event *e;

    if (!sk)
        return 0;

    daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);

    e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e)
        return 0;

    e->daddr = daddr;

    bpf_ringbuf_submit(e, 0);
    return 0;
}

char _license[] SEC("license") = "Dual BSD/GPL";
