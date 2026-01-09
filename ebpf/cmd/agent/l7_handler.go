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

// HTTPEvent represents an L7 HTTP request event from eBPF
// Must match struct http_event in bpf/common/types.h
type HTTPEvent struct {
	Saddr  uint32   // Source IP (client, network byte order)
	Daddr  uint32   // Destination IP (local, network byte order)
	Sport  uint16   // Source Port (client)
	Dport  uint16   // Destination Port (local)
	Pid    uint32   // Process ID
	Comm   [16]byte // Process name
	Method [8]byte  // HTTP Method
	Path   [64]byte // Request Path (depth limited)
}

const httpEventSize = 104 // sizeof(HTTPEvent): 4+4+2+2+4+16+8+64 = 104

// L7Handler processes L7 HTTP request events
type L7Handler struct {
	reader  *ringbuf.Reader
	mapper  *k8smapper.Mapper
	counter *prometheus.CounterVec
	filter  *HealthCheckFilter
}

// HealthCheckFilter filters out health check requests
type HealthCheckFilter struct {
	patterns []string
	enabled  bool
}

// Default health check path patterns
var defaultHealthCheckPatterns = []string{
	"/healthz",
	"/readyz",
	"/livez",
	"/health",
	"/ready",
	"/live",
	"/ping",
	"/status",
	"/_health",
}

// NewHealthCheckFilter creates a health check filter
func NewHealthCheckFilter(enabled bool, customPatterns string) *HealthCheckFilter {
	patterns := make([]string, len(defaultHealthCheckPatterns))
	copy(patterns, defaultHealthCheckPatterns)

	// Add custom patterns
	if customPatterns != "" {
		for _, p := range strings.Split(customPatterns, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				patterns = append(patterns, strings.ToLower(p))
			}
		}
	}

	return &HealthCheckFilter{
		patterns: patterns,
		enabled:  enabled,
	}
}

// IsHealthCheck checks if the path is a health check endpoint
func (f *HealthCheckFilter) IsHealthCheck(path string) bool {
	if !f.enabled {
		return false
	}

	pathLower := strings.ToLower(path)
	for _, pattern := range f.patterns {
		if strings.HasPrefix(pathLower, pattern) {
			return true
		}
	}
	return false
}

// Patterns returns the list of health check patterns
func (f *HealthCheckFilter) Patterns() []string {
	return f.patterns
}

// NewL7Handler creates a new L7 HTTP event handler
func NewL7Handler(reader *ringbuf.Reader, mapper *k8smapper.Mapper, counter *prometheus.CounterVec, filter *HealthCheckFilter) *L7Handler {
	return &L7Handler{
		reader:  reader,
		mapper:  mapper,
		counter: counter,
		filter:  filter,
	}
}

// Run starts the L7 event processing loop
func (h *L7Handler) Run(ctx context.Context) error {
	log.Println("[L7] Handler started; waiting for HTTP request events")

	// Log filter configuration
	if h.filter.enabled {
		log.Printf("[L7] Health check filter enabled, patterns=%v", h.filter.Patterns())
	} else {
		log.Println("[L7] Health check filter disabled")
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("[L7] Handler stopped")
			return ctx.Err()
		default:
		}

		record, err := h.reader.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				log.Println("[L7] ringbuf reader closed; exiting")
				return nil
			}
			log.Printf("[L7] ringbuf read error: %v", err)
			continue
		}

		h.processRecord(record)
	}
}

// processRecord handles a single HTTP event record
func (h *L7Handler) processRecord(record ringbuf.Record) {
	if len(record.RawSample) < httpEventSize {
		log.Printf("[L7] decode error: short sample (%d bytes, want %d)", len(record.RawSample), httpEventSize)
		return
	}

	event := h.parseEvent(record.RawSample)

	// Extract strings from byte arrays
	method := bytesToString(event.Method[:])
	path := bytesToString(event.Path[:])
	comm := bytesToString(event.Comm[:])

	// Convert IPs
	srcIP := uint32ToIP(event.Saddr)
	// dstIP := uint32ToIP(event.Daddr) // Not used currently

	// Check health check filter (completely exclude from metrics)
	if h.filter.IsHealthCheck(path) {
		log.Printf("[L7][HEALTHCHECK] src=%s method=%s path=%s comm=%s (excluded)", srcIP, method, path, comm)
		return
	}

	// Lookup destination K8s metadata using local IP
	// Note: In current implementation, daddr may be 0, so we use process-based lookup
	var ns, svc, pod string = "unknown", "unknown", "unknown"

	// Try to get metadata from the destination (if available)
	if event.Daddr != 0 {
		dstIP := uint32ToIP(event.Daddr)
		if meta, ok := h.mapper.Lookup(dstIP.String()); ok {
			ns = meta.Namespace
			svc = meta.Service
			pod = meta.Pod
		}
	}

	// Update metrics
	h.counter.WithLabelValues(
		srcIP.String(), // source_ip (client)
		ns,             // destination_namespace
		svc,            // destination_service
		pod,            // destination_pod
		method,         // method
		path,           // path (depth limited by eBPF)
		comm,           // process_comm
	).Inc()

	log.Printf("[L7][COUNTED] src=%s ns=%s svc=%s pod=%s method=%s path=%s comm=%s",
		srcIP, ns, svc, pod, method, path, comm)
}

// parseEvent parses raw bytes into HTTPEvent struct
func (h *L7Handler) parseEvent(raw []byte) HTTPEvent {
	var event HTTPEvent

	event.Saddr = binary.LittleEndian.Uint32(raw[0:4])
	event.Daddr = binary.LittleEndian.Uint32(raw[4:8])
	event.Sport = binary.LittleEndian.Uint16(raw[8:10])
	event.Dport = binary.LittleEndian.Uint16(raw[10:12])
	event.Pid = binary.LittleEndian.Uint32(raw[12:16])
	copy(event.Comm[:], raw[16:32])
	copy(event.Method[:], raw[32:40])
	copy(event.Path[:], raw[40:104])

	return event
}

// Close closes the ring buffer reader
func (h *L7Handler) Close() error {
	return h.reader.Close()
}

// bytesToString converts a null-terminated byte array to string
func bytesToString(b []byte) string {
	if idx := bytes.IndexByte(b, 0); idx != -1 {
		return string(b[:idx])
	}
	return string(b)
}

// uint32ToIP converts a uint32 (network byte order) to net.IP
func uint32ToIP(addr uint32) net.IP {
	ip := make(net.IP, net.IPv4len)
	// eBPF stores in network byte order, so we use BigEndian
	binary.BigEndian.PutUint32(ip, addr)
	return ip
}
