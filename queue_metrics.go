package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"go-microservice/utils"
)

type queueCollector struct {
	d                        *utils.Dispatcher
	depth, processed, failed *prometheus.Desc
}

func newQueueCollector(d *utils.Dispatcher) *queueCollector {
	return &queueCollector{d, prometheus.NewDesc("async_queue_depth", "Pending events", nil, nil), prometheus.NewDesc("async_processed_total", "Completed events", nil, nil), prometheus.NewDesc("async_backup_failures_total", "Failed events after retries", nil, nil)}
}
func (c *queueCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.depth
	ch <- c.processed
	ch <- c.failed
}
func (c *queueCollector) Collect(ch chan<- prometheus.Metric) {
	n, p, f := c.d.Stats()
	ch <- prometheus.MustNewConstMetric(c.depth, prometheus.GaugeValue, float64(n))
	ch <- prometheus.MustNewConstMetric(c.processed, prometheus.CounterValue, float64(p))
	ch <- prometheus.MustNewConstMetric(c.failed, prometheus.CounterValue, float64(f))
}
