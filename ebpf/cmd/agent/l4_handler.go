package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"log"
	"net"
	"strings"

	"github.com/cilium/ebpf/ringbuf"
	"github.com/prometheus/client_golang/prometheus"

	"ebpf-k8s-internal-traffic-metrics/internal/k8smapper"
)

// L4Event represents an L4 outbound TCP connection event from eBPF
type L4Event struct {
	Daddr uint32   // Destination IPv4 (network byte order)
	Comm  [16]byte // Process name
}

const l4EventSize = 20 // sizeof(L4Event)

// L4Handler processes L4 outbound TCP connection events
type L4Handler struct {
	reader  *ringbuf.Reader
	mapper  *k8smapper.Mapper
	counter *prometheus.CounterVec
	filter  *L4Filter
}

// L4Filter configuration for L4 events
type L4Filter struct {
	excludeComms map[string]struct{} // Process names to exclude (prefix match)
}

// NewL4Filter creates a new L4 filter with default and custom exclusions
func NewL4Filter(customExcludeComms string) *L4Filter {
	f := &L4Filter{
		excludeComms: make(map[string]struct{}),
	}

	// Default exclusions
	f.excludeComms["kubelet"] = struct{}{}

	// Add custom exclusions
	for _, part := range strings.Split(customExcludeComms, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			f.excludeComms[strings.ToLower(part)] = struct{}{}
		}
	}

	return f
}

// ShouldExclude checks if the process should be excluded (prefix match)
func (f *L4Filter) ShouldExclude(comm string) (bool, string) {
	commLower := strings.ToLower(comm)
	for prefix := range f.excludeComms {
		if strings.HasPrefix(commLower, prefix) {
			return true, prefix
		}
	}
	return false, ""
}

// ExcludeCommsList returns list of excluded comm prefixes for logging
func (f *L4Filter) ExcludeCommsList() []string {
	list := make([]string, 0, len(f.excludeComms))
	for k := range f.excludeComms {
		list = append(list, k)
	}
	return list
}

// NewL4Handler creates a new L4 event handler
func NewL4Handler(reader *ringbuf.Reader, mapper *k8smapper.Mapper, counter *prometheus.CounterVec, filter *L4Filter) *L4Handler {
	return &L4Handler{
		reader:  reader,
		mapper:  mapper,
		counter: counter,
		filter:  filter,
	}
}

// Run starts the L4 event processing loop
func (h *L4Handler) Run(ctx context.Context) error {
	log.Println("[L4] Handler started; waiting for TCP connect events")

	// Log filter configuration
	log.Printf("[L4] excludeComms=%v (count=%d)", h.filter.ExcludeCommsList(), len(h.filter.excludeComms))

	for {
		select {
		case <-ctx.Done():
			log.Println("[L4] Handler stopped")
			return ctx.Err()
		default:
		}

		record, err := h.reader.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				log.Println("[L4] ringbuf reader closed; exiting")
				return nil
			}
			log.Printf("[L4] ringbuf read error: %v", err)
			continue
		}

		h.processRecord(record)
	}
}

// processRecord handles a single L4 event record
func (h *L4Handler) processRecord(record ringbuf.Record) {
	if len(record.RawSample) < l4EventSize {
		log.Printf("[L4] decode error: short sample (%d bytes, want %d)", len(record.RawSample), l4EventSize)
		return
	}

	raw := record.RawSample

	// Parse destination address (network byte order)
	addr := binary.BigEndian.Uint32(raw[:4])

	// Parse comm (null-terminated string)
	commRaw := raw[4:l4EventSize]
	comm := string(commRaw)
	if idx := bytes.IndexByte(commRaw, 0); idx != -1 {
		comm = string(commRaw[:idx])
	}

	// Convert to IP
	ip := make(net.IP, net.IPv4len)
	binary.BigEndian.PutUint32(ip, addr)

	// Apply filter
	if excluded, matchedPrefix := h.filter.ShouldExclude(comm); excluded {
		log.Printf("[L4][FILTERED] dest=%s comm=%s matched=%s", ip.String(), comm, matchedPrefix)
		return
	}

	// Lookup K8s metadata
	meta, ok := h.mapper.Lookup(ip.String())
	if !ok {
		log.Printf("[L4][UNMAPPED] dest=%s comm=%s", ip.String(), comm)
		return
	}

	// Extract metadata with defaults
	ns := meta.Namespace
	svc := meta.Service
	pod := meta.Pod
	if ns == "" {
		ns = "unknown"
	}
	if svc == "" {
		svc = "unknown"
	}
	if pod == "" {
		pod = "unknown"
	}

	// Update metrics
	h.counter.WithLabelValues(ns, svc, pod, comm).Inc()
	log.Printf("[L4][COUNTED] dest=%s ns=%s svc=%s pod=%s comm=%s", ip.String(), ns, svc, pod, comm)
}

// Close closes the ring buffer reader
func (h *L4Handler) Close() error {
	return h.reader.Close()
}
