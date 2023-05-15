package main

/* linktap_exporter is a prometheus exporter for linktap wireless water time.
 * metrics are retrieved from the gateway using the local http API.
 *
 * Copyright 2023 Andreas Krebs
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import (
	"net/http"

	"flag"
	"os"

	log "github.com/sirupsen/logrus"
)

var (
	flagGatewayURL = flag.String("gateway-url", "", "The URL of the LinkTap Gateway")
	flagConfigFile = flag.String("config-file", "linktap_exporter.json", "The JSON file with the metric definitions and devices.")
	flagAddr       = flag.String("listen-address", "127.0.0.1:9045", "The address to listen on for HTTP requests.")
	flagLogLevel   = flag.String("log-level", "info", "The log level {trace|debug|info|warn|error}")
)

var config *ExporterConfig
var gatewayHandler *GatewayHandler

func init() {

	flag.Parse()

	// init log level
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{DisableTimestamp: true})

	logLevel, err := log.ParseLevel(*flagLogLevel)
	if err != nil {
		log.Fatalf("error parsing log level:", err)
	} else {
		log.SetLevel(logLevel)
	}
}

func main() {

	if *flagGatewayURL == "" {
		log.Fatal("--gateway-url parameter missing")
	}

	// read config
	var err error
	config, err = ParseConfigJSON(*flagConfigFile)
	if err != nil {
		log.Fatalf("error reading config file:", err)
	}

	// now start http server to serve metrics and handle events
	gatewayHandler = initGatewayHandler(*flagGatewayURL, *flagAddr)
	registerCollectorAndServeMetrics(*flagAddr)

	log.Fatal(http.ListenAndServe(*flagAddr, nil))
}
