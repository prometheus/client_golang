package prometheus

import "github.com/prometheus/client_golang/procfs"

type processCollector struct {
	pid             int
	fn              func(chan<- Metric)
	cpuTotal        Counter
	openFDs, maxFDs Gauge
	vsize, rss      Gauge
	startTime       Gauge
}

// NewProcessCollector returns a collector which exports the current state of
// process metrics including cpu, memory and file descriptor usage as well as
// the process start time for the given process id under the given namespace.
func NewProcessCollector(pid int, namepsace string) *processCollector {
	c := processCollector{
		fn:  noopCollect,
		pid: pid,

		cpuTotal: NewCounter(CounterOpts{
			Namespace: namepsace,
			Name:      "process_cpu_seconds_total",
			Help:      "Total user and system CPU time spent in seconds.",
		}),
		openFDs: NewGauge(GaugeOpts{
			Namespace: namepsace,
			Name:      "process_open_fds",
			Help:      "Number of open file descriptors.",
		}),
		maxFDs: NewGauge(GaugeOpts{
			Namespace: namepsace,
			Name:      "process_max_fds",
			Help:      "Maximum number of open file descriptors.",
		}),
		vsize: NewGauge(GaugeOpts{
			Namespace: namepsace,
			Name:      "process_virtual_memory_bytes",
			Help:      "Virtual memory size in bytes.",
		}),
		rss: NewGauge(GaugeOpts{
			Namespace: namepsace,
			Name:      "process_resident_memory_bytes",
			Help:      "Resident memory size in bytes.",
		}),
		startTime: NewGauge(GaugeOpts{
			Namespace: namepsace,
			Name:      "process_start_time_seconds",
			Help:      "Start time of the process since unix epoch in seconds.",
		}),
	}

	// Use procfs to export metrics if available.
	if _, err := procfs.Stat(); err == nil {
		c.fn = c.procfsCollect
	}

	return &c
}

// Describe returns all descriptions of the collector.
func (c *processCollector) Describe(ch chan<- *Desc) {
	c.cpuTotal.Describe(ch)
	c.openFDs.Describe(ch)
	c.maxFDs.Describe(ch)
	c.vsize.Describe(ch)
	c.rss.Describe(ch)
	c.startTime.Describe(ch)
}

// Collect returns the current state of all metrics of the collector.
func (c *processCollector) Collect(ch chan<- Metric) {
	c.fn(ch)
}

func noopCollect(ch chan<- Metric) {}

func (c *processCollector) procfsCollect(ch chan<- Metric) {
	p, err := procfs.Process(c.pid)
	if err != nil {
		return
	}

	if stat, err := p.Stat(); err == nil {
		c.cpuTotal.Set(stat.CPUTime())
		c.cpuTotal.Collect(ch)
		c.vsize.Set(float64(stat.VirtualMemory()))
		c.vsize.Collect(ch)
		c.rss.Set(float64(stat.ResidentMemory()))
		c.rss.Collect(ch)

		if startTime, err := stat.StartTime(); err == nil {
			c.startTime.Set(startTime)
			c.startTime.Collect(ch)
		}
	}

	if fds, err := p.FileDescriptorsLen(); err == nil {
		c.openFDs.Set(float64(fds))
		c.openFDs.Collect(ch)
	}

	if limits, err := p.Limits(); err == nil {
		c.maxFDs.Set(float64(limits.OpenFiles))
		c.maxFDs.Collect(ch)
	}
}
