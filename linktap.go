package main

// config structs and JSON support

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

/* device status as defined in MQTT Client Interaction Messages for CMD:3, eg:
{
"dev_id":"1111222233334444",
"plan_mode":2,
"plan_sn":3134,
"is_rf_linked":true,
"is_flm_plugin":false,
"is_fall":false,
"is_broken":false,
"is_cutoff":false,
"is_leak":false,
"is_clog":false,
"signal":100,
"battery":0,
"child_lock":0,
"is_manual_mode":false,
"is_watering":false,
"is_final":true,
"total_duration":0,
"remain_duration":0,
"speed":0,
"volume":0
}
*/

const query_gw_id = "{\"cmd\":3}"
const query_gw_config = "{\"cmd\":16,\"gw_id\":\"%s\"}"
const query_device_status = "{\"cmd\":3,\"gw_id\":\"%s\",\"dev_id\":\"%s\"}"

type DeviceStatus struct {
	Values map[string]interface{}
	Time   int64
	Count  int64
}

type GatewayHandler struct {
	sync.RWMutex
	gatewayURL     string
	gw_id          string
	gw_unit        string
	gw_version     string
	gw_device_list []string
	gw_msg_count   int64
	devices        map[string]*DeviceStatus
}

func (gh *GatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debugf("received %s from %s", r.Method, r.RemoteAddr)

	if r.Method != http.MethodPost {
		log.Warnf("invalid %s request from %s", r.Method, r.RemoteAddr)
		http.Error(w, "405 Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var ds map[string]interface{}
	err := json.NewDecoder(r.Body).Decode(&ds)

	if err != nil {
		log.Errorf("JSON decode failed: %s", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Debugf("Received Status: %v", ds)

	cmd, ok := ds["cmd"].(float64)
	if !ok {
		// no cmd so we assume normal status msg
		cmd = 3
	}

	if cmd == 0 || cmd == 16 {
		err = gh.handleGateWayConfig(ds, cmd)
	} else if cmd == 3 {
		err = gh.handleDeviceData(ds)
	} else {
		err = fmt.Errorf("unknown cmd %f in json data: %v", cmd, ds)
	}

	if err != nil {
		log.Errorf(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Fprintf(w, "Device Status: %+v", ds)
}

func (gh *GatewayHandler) handleDeviceData(ds map[string]interface{}) error {

	dev_id, ok := ds["dev_id"].(string)
	if !ok {
		return fmt.Errorf("no dev_id in json data: %v", ds)
	}

	ts := time.Now().Unix()

	// now add status values to devices map
	gh.Lock()
	defer gh.Unlock()

	devStat := gh.devices[dev_id]
	if devStat == nil {
		devStat = &DeviceStatus{ds, ts, 1}
		gh.devices[dev_id] = devStat
	} else {
		devStat.Values = ds
		devStat.Time = ts
		devStat.Count++
	}

	return nil
}

func (gh *GatewayHandler) pollDevices() {
	for _, dev_id := range gh.gw_device_list {

		ds, err := gh.gatewayRequest(fmt.Sprintf(query_device_status, gh.gw_id, dev_id))
		if err != nil {
			log.Errorf("failed to get device status for gw_id=%s, dev_id=%s: %s", gh.gw_id, dev_id, err.Error())
			continue
		}

		id, ok := ds["dev_id"].(string)
		if !ok {
			log.Errorf("no dev_id in json data: %v", ds)
			continue
		} else if id != dev_id {
			log.Errorf("wrong device id in status, expected %s but received: %v", dev_id, ds)
			continue
		}

		log.Infof("retrieved device status for gw_id=%s, dev_id=%s", gh.gw_id, dev_id)

		err = gh.handleDeviceData(ds)
		if err != nil {
			log.Errorf(err.Error())
		}
	}
}

func (gh *GatewayHandler) gatewayRequest(payload string) (map[string]interface{}, error) {

	log.Debugf("%s/api.shtml POST %s", gh.gatewayURL, payload)
	resp, err := http.Post(gh.gatewayURL+"/api.shtml", "application/json", strings.NewReader(payload))

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	// now find JSON in response HTML
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	log.Debugf("Received: %s", string(b))
	startIndex := bytes.IndexByte(b, '{')
	endIndex := bytes.LastIndexByte(b, '}')

	if startIndex == -1 || endIndex == -1 {
		return nil, errors.New("failed to find { } in gateway response")
	}

	var res map[string]interface{}
	err = json.Unmarshal(b[startIndex:endIndex+1], &res)
	log.Debugf("range: %s", string(b[startIndex:endIndex]))
	if err != nil {
		log.Errorf("JSON decode failed: %s", err.Error())
		return nil, err
	}

	return res, nil
}

func (gh *GatewayHandler) getGatewayConfig() {

	// set defauls - in case config API is not working
	gh.gw_unit = "liter"
	gh.gw_version = "unknown"
	gh.gw_msg_count = 0
	gh.gw_device_list = make([]string, 0, 2)

	ds, err := gh.gatewayRequest(fmt.Sprintf(query_gw_config, gh.gw_id))
	if err != nil {
		log.Errorf("failed to get gateway config for gw_id=%s: %s", gh.gw_id, err.Error())
		return
	}

	cmd, ok := ds["cmd"].(float64)
	if !ok {
		log.Errorf("no cmd in json data: %v", ds["cmd"], ds)
	}

	gh.handleGateWayConfig(ds, cmd)
}

func (gh *GatewayHandler) handleGateWayConfig(ds map[string]interface{}, cmd float64) error {

	id, ok := ds["gw_id"].(string)
	if !ok {
		return fmt.Errorf("no gw_id in json data: %v", ds)
	} else if id != gh.gw_id {
		return fmt.Errorf("wrong gateway id in config, expected %s but received: %v", gh.gw_id, id)
	}

	log.Infof("retrieved gateway config for gw_id=%s: %v", gh.gw_id, ds)

	gh.Lock()
	defer gh.Unlock()

	gh.gw_msg_count++

	ver, ok := ds["ver"].(string)
	if !ok {
		log.Warnf("gateway version (ver) missing in config")
	} else {
		gh.gw_version = ver
	}

	if cmd == 16 {
		unit, ok := ds["vol_unit"].(string)
		if !ok {
			log.Warnf("gateway volume unit (vol_unit) missing in config")
		} else if unit == "gal" {
			gh.gw_unit = "gallon"
		} else if unit == "L" {
			gh.gw_unit = "liter"
		} else {
			log.Warnf("gateway volume unit '%s' is unknown", unit)
		}
	}

	devs, ok := ds["end_dev"].([]interface{})
	if !ok {
		log.Warnf("connected devices (end_dev: %v) missing in config", ds["end_dev"])
	} else {
		dev_list := make([]string, 0, len(devs))
		for _, dev_id_untyped := range devs {
			dev_id, ok := dev_id_untyped.(string)
			if !ok {
				log.Warnf("invalid device ID %v in config", dev_id_untyped)

			} else {
				dev_list = append(dev_list, dev_id)
			}
		}

		if len(dev_list) > 0 {
			gh.gw_device_list = dev_list
			log.Infof("using device list retrieved from gateway config: %v", dev_list)
		} else {
			log.Warnf("failed to get any devices from device list: %s", devs)
		}
	}

	log.Debugf("gateway handler: %v", gh)
	return nil
}

func initGatewayHandler(gatewayURL string, addr string) *GatewayHandler {
	gh := &GatewayHandler{gatewayURL: gatewayURL}
	gh.devices = make(map[string]*DeviceStatus)

	// first get gateway ID by sending plain cmd:3
	res, err := gh.gatewayRequest(query_gw_id)
	if err != nil {
		log.Fatalf("failed to get gateway id using URL %s: %s", gatewayURL, err.Error())
	}

	var ok bool
	gh.gw_id, ok = res["gw_id"].(string)
	if !ok {
		log.Fatalf("no gw_id in json data: %v", res)
	}
	log.Infof("retrieved gateway Id: %s", gh.gw_id)

	// now try to get gateway config
	gh.getGatewayConfig()

	// gateway API is working, so request status of devices
	go gh.pollDevices()

	http.Handle("/gateway", gh)
	log.Infof("gateway receiver started - configure http://%s/gateway as HTTP API", addr)

	return gh
}
