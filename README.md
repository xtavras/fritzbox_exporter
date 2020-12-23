# Fritz!Box Upnp statistics exporter for prometheus

This exporter exports some variables from an 
[AVM Fritzbox](http://avm.de/produkte/fritzbox/)
to prometheus.

This exporter is tested with a Fritzbox 4040 software version 07.14.

## Building

    go get github.com/aexel90/fritzbox_exporter/
    cd $GOPATH/src/github.com/aexel90/fritzbox_exporter
    go install

## Running

In the configuration of the Fritzbox the option "Statusinformationen über UPnP übertragen" in the dialog "Heimnetz >
Heimnetzübersicht > Netzwerkeinstellungen" has to be enabled.

Usage:

    $GOPATH/bin/fritzbox_exporter -h
    Usage of ./fritzbox_exporter:
    -collect-upnp
        If set ALL available upnp metrics will be collected
    -gateway-lua-url string
        The URL of the FRITZ!Box - LUA (default "http://fritz.box")
    -gateway-upnp-url string
        The URL of the FRITZ!Box - UPNP (default "http://fritz.box:49000")
    -listen-address string
        The address to listen on for HTTP requests. (default "127.0.0.1:9042")
    -metrics-lua string
        The JSON file with the lua metric definitions.
    -metrics-upnp string
        The JSON file with the upnp metric definitions.
    -password string
        The password for the FRITZ!Box
    -result-file-upnp string
        The JSON file where to store upnp export results during test
    -result-file-upnp-all string
        The JSON file where to store the result during collect
    -result-file-lua string
        The JSON file where to store lua export results during test
    -test
        test configured metrics
    -username string
        The user for the FRITZ!Box UPnP service
    
## Example execution

### Running within prometheus:

    $GOPATH/bin/fritzbox_exporter -username <username> -password <password> -metrics-upnp $GOPATH/bin/metrics-upnp.json -metrics-lua $GOPATH/bin/metrics-lua.json
    
### Test exporter with upnp metrics and result file storage:

    $GOPATH/bin/fritzbox_exporter -username <username> -password <password> -test -metrics-upnp $GOPATH/bin/metrics-upnp.json -result-file-upnp $GOPATH/bin/result-upnp.json

### Test exporter with lua metrics and result file storage:

    $GOPATH/bin/fritzbox_exporter -username <username> -password <password> -test -metrics-lua $GOPATH/bin/metrics-lua.json -result-file-lua $GOPATH/bin/result-lua.json

### Print all available upnp metrics and its results

    $GOPATH/bin/fritzbox_exporter -username <username> -password <password> -collect-upnp -result-file-upnp-all $GOPATH/bin/result-upnp-collect.json

## Grafana Dashboard

The dashboard is published here [Grafana](https://grafana.com/grafana/dashboards/13377).

Dashboard ID is 13377.

![Grafana](https://raw.githubusercontent.com/aexel90/fritzbox_exporter/master/grafana/screenshot.jpg)