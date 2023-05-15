package main

// prometheus metrics creation and collection

import (
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	log "github.com/sirupsen/logrus"
)

const COUNTER_METRIC = "counter"

var metricDescs map[string]*prometheus.Desc
var metricTypes map[string]prometheus.ValueType
var metricLabels []string
var configLabelIndex map[string]int

type LinktapCollector struct {
}

// Implement prometheus Collector
func (lc *LinktapCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, md := range metricDescs {
		ch <- md
	}
}

func valueAsFloat(val interface{}) (float64, error) {
	switch v := val.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case bool:
		if v {
			return 1.0, nil
		} else {
			return 0.0, nil
		}
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return math.NaN(), fmt.Errorf("can't convert %v to float64", val)
	}
}

func (lc *LinktapCollector) Collect(ch chan<- prometheus.Metric) {

	gatewayHandler.RLock()
	defer gatewayHandler.RUnlock()

	for dev_id, dev := range gatewayHandler.devices {

		labels := []string{gatewayHandler.gw_id, gatewayHandler.gw_unit, dev_id, config.Devices[dev_id].Name}

		for metric, md := range metricDescs {
			vt := metricTypes[metric]

			var fval float64
			if metric == COUNTER_METRIC {
				fval = float64(dev.Count)
			} else {

				val := dev.Values[metric]
				if val == nil {
					continue
				}

				var err error
				fval, err = valueAsFloat(val)
				if err != nil {
					log.Warnf("error converting metric %s: %s", metric, err.Error())
					continue
				}
			}

			metric, err := prometheus.NewConstMetric(md, vt, fval, labels...)
			if err != nil {
				log.Errorf("Error creating metric %s", err)
			} else {
				ch <- metric
			}
		}
	}
}

func createMetricDescriptions() {

	metrics := config.Metrics

	metricDescs = make(map[string]*prometheus.Desc, len(metrics)+1)
	metricTypes = make(map[string]prometheus.ValueType, len(metrics)+1)

	// default metrics
	metricLabels = []string{"gw_id", "gw_unit", "dev_id", "dev_name"}

	log.Debugf("metric labels: %v", metricLabels)
	log.Debugf("configLabelIndex: %v", configLabelIndex)

	for _, mt := range metrics {

		metricDescs[mt.Id] = prometheus.NewDesc("linktap_watertimer_"+mt.Id, mt.Help, metricLabels, nil)

		var vt prometheus.ValueType

		if mt.Type == "counter" {
			vt = prometheus.CounterValue
		} else if mt.Type == "gauge" || mt.Type == "flag" {
			vt = prometheus.GaugeValue
		} else {
			vt = prometheus.UntypedValue
		}

		metricTypes[mt.Id] = vt
	}

	// add counter metric
	metricDescs[COUNTER_METRIC] = prometheus.NewDesc("linktap_watertimer_msg_counter", "number of status messages received for device", metricLabels, nil)
	metricTypes[COUNTER_METRIC] = prometheus.CounterValue
}

func registerCollectorAndServeMetrics(addr string) {

	createMetricDescriptions()
	ltCol := &LinktapCollector{}

	prometheus.MustRegister(ltCol)

	// now start http server and serve metrics
	http.Handle("/metrics", promhttp.Handler())
	log.Infof("Exporter started - metrics available at http://%s/metrics", addr)
}
