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

const METRIC_DEV_MSG_COUNT = "dev_msg_count"
const METRIC_GW_DEV_COUNT = "gw_dev_count"
const METRIC_GW_VERSION = "gw_version"
const METRIC_GW_MSG_COUNT = "gw_msg_count"

var metricDescs map[string]*prometheus.Desc
var gwMetricDescs map[string]*prometheus.Desc
var metricTypes map[string]prometheus.ValueType

var metricLabels []string
var configLabelIndex map[string]int

type LinktapCollector struct {
}

// Implement prometheus Collector
func (lc *LinktapCollector) Describe(ch chan<- *prometheus.Desc) {

	for _, md := range gwMetricDescs {
		ch <- md
	}

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

	// report gateway metrics
	labels := []string{gatewayHandler.gw_id, gatewayHandler.gw_unit}
	metric, err := prometheus.NewConstMetric(gwMetricDescs[METRIC_GW_DEV_COUNT], metricTypes[METRIC_GW_DEV_COUNT], float64(len(gatewayHandler.gw_device_list)), labels...)
	if err != nil {
		log.Errorf("Error creating metric %s", err)
	} else {
		ch <- metric
	}

	metric, err = prometheus.NewConstMetric(gwMetricDescs[METRIC_GW_MSG_COUNT], metricTypes[METRIC_GW_MSG_COUNT], float64(gatewayHandler.gw_msg_count), labels...)
	if err != nil {
		log.Errorf("Error creating metric %s", err)
	} else {
		ch <- metric
	}

	labels = append(labels, gatewayHandler.gw_version)
	metric, err = prometheus.NewConstMetric(gwMetricDescs[METRIC_GW_VERSION], metricTypes[METRIC_GW_VERSION], 1.0, labels...)
	if err != nil {
		log.Errorf("Error creating metric %s", err)
	} else {
		ch <- metric
	}

	// report device metrics
	for dev_id, dev := range gatewayHandler.devices {

		labels := []string{gatewayHandler.gw_id, gatewayHandler.gw_unit, dev_id}

		for _, dev_labels := range config.DeviceLabels {

			value := ""
			for _, val_pattern := range dev_labels.ValuePatterns {

				if val_pattern.IdRegex.MatchString(dev_id) {
					value = val_pattern.Value
					break
				}
			}

			labels = append(labels, value)
		}

		log.Debugf("device_labels: %v", labels)

		for metric, md := range metricDescs {
			vt := metricTypes[metric]

			var fval float64
			if metric == METRIC_DEV_MSG_COUNT {
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
	gwMetricDescs = make(map[string]*prometheus.Desc, 2)
	metricTypes = make(map[string]prometheus.ValueType, len(metrics)+3)

	// default label
	metricLabels = []string{"gw_id", "gw_unit", "dev_id"}

	for _, dev_labels := range config.DeviceLabels {
		metricLabels = append(metricLabels, dev_labels.Label)
	}

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

	// add counter metrics
	metricDescs[METRIC_DEV_MSG_COUNT] = prometheus.NewDesc("linktap_watertimer_msg_counter", "number of status messages received for device", metricLabels, nil)
	metricTypes[METRIC_DEV_MSG_COUNT] = prometheus.CounterValue

	// create gateway metrics
	gwMetricDescs[METRIC_GW_DEV_COUNT] = prometheus.NewDesc("linktap_gateway_connected_devices", "number of devices connected to gateway", []string{"gw_id", "gw_unit"}, nil)
	metricTypes[METRIC_GW_DEV_COUNT] = prometheus.GaugeValue

	gwMetricDescs[METRIC_GW_MSG_COUNT] = prometheus.NewDesc("linktap_gateway_msg_counter", "number of status messages received for gateway", []string{"gw_id", "gw_unit"}, nil)
	metricTypes[METRIC_GW_MSG_COUNT] = prometheus.CounterValue

	gwMetricDescs[METRIC_GW_VERSION] = prometheus.NewDesc("linktap_gateway_version", "metric for storing gateway version as label, always 1", []string{"gw_id", "gw_unit", "gw_version"}, nil)
	metricTypes[METRIC_GW_VERSION] = prometheus.GaugeValue
}

func registerCollectorAndServeMetrics(addr string) {

	createMetricDescriptions()
	ltCol := &LinktapCollector{}

	prometheus.MustRegister(ltCol)

	// now start http server and serve metrics
	http.Handle("/metrics", promhttp.Handler())
	log.Infof("Exporter started - metrics available at http://%s/metrics", addr)
}
