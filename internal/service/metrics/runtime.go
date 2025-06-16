package metrics

import (
	"github.com/unibackend/uniproxy/internal/config"
	"runtime"
	"time"
)

const (
	RuntimeCpuNum           = "runtime_cpu_num"
	RuntimeCpuGoroutine     = "runtime_cpu_goroutine"
	RuntimeCpuCgoCall       = "runtime_cpu_cgo_call"
	RuntimeMemAlloc         = "runtime_mem_alloc"
	RuntimeMemAllocTotal    = "runtime_mem_alloc_total"
	RuntimeMemSys           = "runtime_mem_sys"
	RuntimeMemOthersys      = "runtime_mem_othersys"
	RuntimeMemLookups       = "runtime_mem_lookups"
	RuntimeMemMalloc        = "runtime_mem_malloc"
	RuntimeMemFrees         = "runtime_mem_frees"
	RuntimeMemHeapAlloc     = "runtime_mem_heap_alloc"
	RuntimeMemHeapSys       = "runtime_mem_heap_sys"
	RuntimeMemHeapIdle      = "runtime_mem_heap_idle"
	RuntimeMemHeapInuse     = "runtime_mem_heap_inuse"
	RuntimeMemHeapReleased  = "runtime_mem_heap_released"
	RuntimeMemHeapObjects   = "runtime_mem_heap_objects"
	RuntimeMemStackInuse    = "runtime_mem_stack_inuse"
	RuntimeMemStackSys      = "runtime_mem_stack_sys"
	RuntimeMemMspanInuse    = "runtime_mem_mspan_inuse"
	RuntimeMemMspanSys      = "runtime_mem_mspan_sys"
	RuntimeMemMcacheInuse   = "runtime_mem_mcache_inuse"
	RuntimeMemMcacheSys     = "runtime_mem_mcache_sys"
	RuntimeMemGCSys         = "runtime_mem_gc_sys"
	RuntimeMemGCNext        = "runtime_mem_gc_next"
	RuntimeMemGCLast        = "runtime_mem_gc_last"
	RuntimeMemGCPauseTotal  = "runtime_mem_gc_pause_total"
	RuntimeMemGCPause       = "runtime_mem_gc_pause"
	RuntimeMemGCNum         = "runtime_mem_gc_num"
	RuntimeMemGCCount       = "runtime_mem_gc_count"
	RuntimeMemGCCPUFraction = "runtime_mem_gc_cpu_fraction"
)

type runtimeMetrics struct {
	// CPU
	NumCpu       int64 `json:"cpu.count"`
	NumGoroutine int64 `json:"cpu.goroutines"`
	NumCgoCall   int64 `json:"cpu.cgo_calls"`

	// General
	Alloc      int64 `json:"mem.alloc"`
	TotalAlloc int64 `json:"mem.total"`
	Sys        int64 `json:"mem.sys"`
	Lookups    int64 `json:"mem.lookups"`
	Mallocs    int64 `json:"mem.malloc"`
	Frees      int64 `json:"mem.frees"`

	// Heap
	HeapAlloc    int64 `json:"mem.heap.alloc"`
	HeapSys      int64 `json:"mem.heap.sys"`
	HeapIdle     int64 `json:"mem.heap.idle"`
	HeapInuse    int64 `json:"mem.heap.inuse"`
	HeapReleased int64 `json:"mem.heap.released"`
	HeapObjects  int64 `json:"mem.heap.objects"`

	// Stack
	StackInuse  int64 `json:"mem.stack.inuse"`
	StackSys    int64 `json:"mem.stack.sys"`
	MSpanInuse  int64 `json:"mem.stack.mspan_inuse"`
	MSpanSys    int64 `json:"mem.stack.mspan_sys"`
	MCacheInuse int64 `json:"mem.stack.mcache_inuse"`
	MCacheSys   int64 `json:"mem.stack.mcache_sys"`

	OtherSys int64 `json:"mem.othersys"`

	// GC
	GCSys         int64   `json:"mem.gc.sys"`
	NextGC        int64   `json:"mem.gc.next"`
	LastGC        int64   `json:"mem.gc.last"`
	PauseTotalNs  int64   `json:"mem.gc.pause_total"`
	PauseNs       int64   `json:"mem.gc.pause"`
	NumGC         int64   `json:"mem.gc.count"`
	GCCPUFraction float64 `json:"mem.gc.cpu_fraction"`

	Goarch  string `json:"-"`
	Goos    string `json:"-"`
	Version string `json:"-"`
}

type runtimeMetricsCollector struct {
	config   config.Metrics
	measurer Measurer
	Done     <-chan struct{}
}

func (m *runtimeMetricsCollector) Run() {
	m.collect()

	var durationInterval time.Duration
	var err error
	if durationInterval, err = time.ParseDuration(m.config.Runtime.Duration); err != nil {
		durationInterval = 5 * time.Second
	}
	tick := time.NewTicker(durationInterval)
	defer tick.Stop()
	for {
		select {
		case <-m.Done:
			return
		case <-tick.C:
			m.collect()
		}
	}
}

func (m *runtimeMetricsCollector) collect() {
	if m.config.Runtime.CPU {
		m.measurer.Gauge(RuntimeCpuNum).Set(float64(runtime.NumCPU()))
		m.measurer.Gauge(RuntimeCpuGoroutine).Set(float64(runtime.NumGoroutine()))
		m.measurer.Gauge(RuntimeCpuCgoCall).Set(float64(runtime.NumCgoCall()))
	}

	if m.config.Runtime.Mem {
		ms := &runtime.MemStats{}
		runtime.ReadMemStats(ms)

		// General
		m.measurer.Gauge(RuntimeMemAlloc).Set(float64(ms.Alloc))
		m.measurer.Counter(RuntimeMemAllocTotal).Add(float64(ms.TotalAlloc))
		m.measurer.Gauge(RuntimeMemSys).Set(float64(ms.Sys))
		m.measurer.Gauge(RuntimeMemLookups).Set(float64(ms.Lookups))
		m.measurer.Gauge(RuntimeMemMalloc).Set(float64(ms.Mallocs))
		m.measurer.Gauge(RuntimeMemFrees).Set(float64(ms.Frees))

		// Heap
		m.measurer.Gauge(RuntimeMemHeapAlloc).Set(float64(ms.HeapAlloc))
		m.measurer.Gauge(RuntimeMemHeapSys).Set(float64(ms.HeapSys))
		m.measurer.Gauge(RuntimeMemHeapIdle).Set(float64(ms.HeapIdle))
		m.measurer.Gauge(RuntimeMemHeapInuse).Set(float64(ms.HeapInuse))
		m.measurer.Gauge(RuntimeMemHeapReleased).Set(float64(ms.HeapReleased))
		m.measurer.Gauge(RuntimeMemHeapObjects).Set(float64(ms.HeapObjects))

		// Stack
		m.measurer.Gauge(RuntimeMemStackInuse).Set(float64(ms.StackInuse))
		m.measurer.Gauge(RuntimeMemStackSys).Set(float64(ms.StackSys))

		m.measurer.Gauge(RuntimeMemMspanInuse).Set(float64(ms.MSpanInuse))
		m.measurer.Gauge(RuntimeMemMspanSys).Set(float64(ms.MSpanSys))
		m.measurer.Gauge(RuntimeMemMcacheInuse).Set(float64(ms.MCacheInuse))
		m.measurer.Gauge(RuntimeMemMcacheSys).Set(float64(ms.MCacheSys))

		m.measurer.Gauge(RuntimeMemOthersys).Set(float64(ms.OtherSys))

		if m.config.Runtime.GC {
			m.measurer.Gauge(RuntimeMemGCSys).Set(float64(ms.GCSys))
			m.measurer.Gauge(RuntimeMemGCNext).Set(float64(ms.NextGC))
			m.measurer.Gauge(RuntimeMemGCLast).Set(float64(ms.LastGC))
			m.measurer.Gauge(RuntimeMemGCPauseTotal).Set(float64(ms.PauseTotalNs))
			m.measurer.Gauge(RuntimeMemGCPause).Set(float64(ms.PauseNs[(ms.NumGC+255)%256]))

			m.measurer.Gauge(RuntimeMemGCNum).Set(float64(ms.NumGC))
			m.measurer.Gauge(RuntimeMemGCCPUFraction).Set(ms.GCCPUFraction)
		}
	}
}
