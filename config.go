package main

// config structs and JSON support

import (
	"encoding/json"
	"io/ioutil"
	"regexp"

	log "github.com/sirupsen/logrus"
)

type DeviceLabelValueConfig struct {
	Value         string `json:"value"`
	IdRegexString string `json:"id_regex"`
	IdRegex       *regexp.Regexp
}
type DeviceLabelConfig struct {
	Label         string                    `json:"label"`
	ValuePatterns []*DeviceLabelValueConfig `json:"value_patterns"`
}

type MetricConfig struct {
	Id   string `json:"id"`
	Type string `json:"type"`
	Help string `json:"help"`
}

type ExporterConfig struct {
	DeviceLabels []*DeviceLabelConfig `json:"device_labels"`
	Metrics      []*MetricConfig      `json:"metrics"`
}

func ParseConfigJSON(cfgFile string) (*ExporterConfig, error) {

	// read metrics
	jsonData, err := ioutil.ReadFile(*flagConfigFile)
	if err != nil {
		return nil, err
	}

	var config *ExporterConfig
	err = json.Unmarshal(jsonData, &config)
	if err != nil {
		return nil, err
	}

	// compile value regex patterns
	for _, dev_labels := range config.DeviceLabels {
		for _, val_patterns := range dev_labels.ValuePatterns {
			val_patterns.IdRegex = regexp.MustCompile(val_patterns.IdRegexString)
		}
	}

	jsonOut, err := json.Marshal(config)
	if err != nil {
		log.Fatalf(err.Error())
	}
	log.Debugf("CONFIG: read config: %s", string(jsonOut))

	return config, nil
}
