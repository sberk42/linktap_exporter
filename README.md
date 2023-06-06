# linktap_exporter
prometheus exporter for linktap water timer

the exporter uses the local http push API of the gateway to receive metrics and exposes them as prometheus metrics

# configure exporter as target for HTTP push API
- access the link admin page by connecting to http://<linktap gateway IP> (check linktap app if you don't know the gateway IP)
- select 'Enable local HTTP API'
- set 'Server URL' to 'http://<exporter-ip>:<exporter-port>/gateway (exporter-ip and exporter-port is the IP and port you plan to run the exporter on)
- press 'Submit'
- reboot the gateway

# run exporter
- start the exporter like this:
  linktap_exporter -gateway-url http://<linktap gateway IP> -listen-address <exporter-ip>:<exporter-port>
- configure prometheus scraper to collect metrics from http://<exporter-ip>:<exporter-port>/metrics




