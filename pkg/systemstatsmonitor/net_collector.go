/*
Copyright 2020 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package systemstatsmonitor

import (
	"fmt"

	ssmtypes "k8s.io/node-problem-detector/pkg/systemstatsmonitor/types"
	"k8s.io/node-problem-detector/pkg/util/metrics"

	"github.com/golang/glog"
	"github.com/prometheus/procfs"
)

type netCollector struct {
	config   *ssmtypes.NetStatsConfig
	procPath string
	recorder *ifaceStatRecorder
}

func NewNetCollectorOrDie(netConfig *ssmtypes.NetStatsConfig, procPath string) *netCollector {
	nc := netCollector{
		config:   netConfig,
		procPath: procPath,
		recorder: newIfaceStatRecorder(),
	}

	nc.mustRegisterMetric(
		metrics.NetDevRxBytes,
		"Cumulative count of bytes received.",
		"Byte",
		metrics.Sum,
		func(stat procfs.NetDevLine) int64 {
			return int64(stat.RxBytes)
		},
	)

	nc.mustRegisterMetric(
		metrics.NetDevRxPackets,
		"Cumulative count of packets received.",
		"1",
		metrics.Sum,
		func(stat procfs.NetDevLine) int64 {
			return int64(stat.RxPackets)
		},
	)

	nc.mustRegisterMetric(
		metrics.NetDevRxErrors,
		"Cumulative count of receive errors encountered.",
		"1",
		metrics.Sum,
		func(stat procfs.NetDevLine) int64 {
			return int64(stat.RxErrors)
		},
	)

	nc.mustRegisterMetric(
		metrics.NetDevRxDropped,
		"Cumulative count of packets dropped while receiving.",
		"1",
		metrics.Sum,
		func(stat procfs.NetDevLine) int64 {
			return int64(stat.RxDropped)
		},
	)

	nc.mustRegisterMetric(
		metrics.NetDevRxFifo,
		"Cumulative count of FIFO buffer errors.",
		"1",
		metrics.Sum,
		func(stat procfs.NetDevLine) int64 {
			return int64(stat.RxFIFO)
		},
	)

	nc.mustRegisterMetric(
		metrics.NetDevRxFrame,
		"Cumulative count of packet framing errors.",
		"1",
		metrics.Sum,
		func(stat procfs.NetDevLine) int64 {
			return int64(stat.RxFrame)
		},
	)

	nc.mustRegisterMetric(
		metrics.NetDevRxCompressed,
		"Cumulative count of compressed packets received by the device driver.",
		"1",
		metrics.Sum,
		func(stat procfs.NetDevLine) int64 {
			return int64(stat.RxCompressed)
		},
	)

	nc.mustRegisterMetric(
		metrics.NetDevRxMulticast,
		"Cumulative count of multicast frames received by the device driver.",
		"1",
		metrics.Sum,
		func(stat procfs.NetDevLine) int64 {
			return int64(stat.RxMulticast)
		},
	)

	nc.mustRegisterMetric(
		metrics.NetDevTxBytes,
		"Cumulative count of bytes transmitted.",
		"Byte",
		metrics.Sum,
		func(stat procfs.NetDevLine) int64 {
			return int64(stat.TxBytes)
		},
	)
	nc.mustRegisterMetric(
		metrics.NetDevTxPackets,
		"Cumulative count of packets transmitted.",
		"1",
		metrics.Sum,
		func(stat procfs.NetDevLine) int64 {
			return int64(stat.TxPackets)
		},
	)
	nc.mustRegisterMetric(
		metrics.NetDevTxErrors,
		"Cumulative count of transmit errors encountered.",
		"1",
		metrics.Sum,
		func(stat procfs.NetDevLine) int64 {
			return int64(stat.TxErrors)
		},
	)
	nc.mustRegisterMetric(
		metrics.NetDevTxDropped,
		"Cumulative count of packets dropped while transmitting.",
		"1",
		metrics.Sum,
		func(stat procfs.NetDevLine) int64 {
			return int64(stat.TxDropped)
		},
	)
	nc.mustRegisterMetric(
		metrics.NetDevTxFifo,
		"Cumulative count of FIFO buffer errors.",
		"1",
		metrics.Sum,
		func(stat procfs.NetDevLine) int64 {
			return int64(stat.TxFIFO)
		},
	)
	nc.mustRegisterMetric(
		metrics.NetDevTxCollisions,
		"Cumulative count of collisions detected on the interface.",
		"1",
		metrics.Sum,
		func(stat procfs.NetDevLine) int64 {
			return int64(stat.TxCollisions)
		},
	)
	nc.mustRegisterMetric(
		metrics.NetDevTxCarrier,
		"Cumulative count of carrier losses detected by the device driver.",
		"1",
		metrics.Sum,
		func(stat procfs.NetDevLine) int64 {
			return int64(stat.TxCarrier)
		},
	)
	nc.mustRegisterMetric(
		metrics.NetDevTxCompressed,
		"Cumulative count of compressed packets transmitted by the device driver.",
		"1",
		metrics.Sum,
		func(stat procfs.NetDevLine) int64 {
			return int64(stat.TxCompressed)
		},
	)

	return &nc
}

func (nc *netCollector) mustRegisterMetric(metricID metrics.MetricID, description, unit string,
	aggregation metrics.Aggregation, exporter func(stat procfs.NetDevLine) int64) {
	metricConfig, ok := nc.config.MetricsConfigs[string(metricID)]
	if !ok {
		glog.Fatalf("Metric config `%q` not found", metricID)
	}
	err := nc.recorder.Register(metricID, metricConfig.DisplayName, description, unit,
		aggregation, []string{interfaceNameLabel}, exporter)
	if err != nil {
		glog.Fatalf("Failed to initialize metric %q: %v", metricID, err)
	}
}

func (nc *netCollector) recordNetDev() {
	fs, err := procfs.NewFS(nc.procPath)
	stats, err := fs.NetDev()
	if err != nil {
		glog.Errorf("Failed to retrieve net dev stat: %v", err)
		return
	}

	for iface, ifaceStats := range stats {
		tags := map[string]string{}
		tags[interfaceNameLabel] = iface

		nc.recorder.RecordWithSameTags(ifaceStats, tags)
	}
}

func (nc *netCollector) collect() {
	if nc == nil {
		return
	}

	nc.recordNetDev()
}

// TODO(@oif): Maybe implements a generic recorder
type ifaceStatRecorder struct {
	collectors map[metrics.MetricID]ifaceStatCollector
}

func newIfaceStatRecorder() *ifaceStatRecorder {
	return &ifaceStatRecorder{collectors: make(map[metrics.MetricID]ifaceStatCollector)}
}

func (r *ifaceStatRecorder) Register(metricID metrics.MetricID, viewName string, description string,
	unit string, aggregation metrics.Aggregation, tagNames []string, exporter func(procfs.NetDevLine) int64) error {
	if _, ok := r.collectors[metricID]; ok {
		// Check duplication
		return fmt.Errorf("metric %q already registered", metricID)
	}
	metric, err := metrics.NewInt64Metric(metricID, viewName, description, unit, aggregation, tagNames)
	if err != nil {
		return err
	}
	r.collectors[metricID] = ifaceStatCollector{
		metric:   metric,
		exporter: exporter,
	}
	return nil
}

func (r ifaceStatRecorder) RecordWithSameTags(stat procfs.NetDevLine, tags map[string]string) {
	// Range all registered collector and record its measurement with same tags
	for metricID, collector := range r.collectors {
		measurement := collector.exporter(stat)
		collector.metric.Record(tags, measurement)
		glog.V(6).Infof("Metric %q record measurement %d with tags %v", metricID, measurement, tags)
	}
}

type ifaceStatCollector struct {
	metric   *metrics.Int64Metric
	exporter func(procfs.NetDevLine) int64
}
