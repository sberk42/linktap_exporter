package main

// config structs and JSON support

import (
	"encoding/json"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
)

type DeviceConfig struct {
	Name string `json:"name"`
}

type MetricConfig struct {
	Id   string `json:"id"`
	Type string `json:"type"`
	Help string `json:"help"`
}

type ExporterConfig struct {
	Devices map[string]DeviceConfig `json:"devices"`
	Metrics []MetricConfig          `json:"metrics"`
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
	log.Debugf("CONFIG: read config: %v", config)

	return config, nil
}
