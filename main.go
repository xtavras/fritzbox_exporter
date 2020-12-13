package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"

	"github.com/namsral/flag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/aexel90/fritzbox_exporter/collector"
	"github.com/aexel90/fritzbox_exporter/metric"
)

var (
	flagGatewayUpnpURL = flag.String("gateway-upnp-url", "http://fritz.box:49000", "The URL of the FRITZ!Box - UPNP")
	flagGatewayLuaURL  = flag.String("gateway-lua-url", "http://fritz.box", "The URL of the FRITZ!Box - LUA")
	flagUsername       = flag.String("username", "", "The user for the FRITZ!Box UPnP service")
	flagPassword       = flag.String("password", "", "The password for the FRITZ!Box UPnP service")
	flagAddress        = flag.String("listen-address", "127.0.0.1:9042", "The address to listen on for HTTP requests.")

	flagMetricsLuaFile  = flag.String("metrics-lua", "", "The JSON file with the lua metric definitions.")
	flagMetricsUpnpFile = flag.String("metrics-upnp", "", "The JSON file with the upnp metric definitions.")

	flagTest = flag.Bool("test", false, "test mode")
)

func main() {

	var metricsFileLua *metric.MetricsFile
	var metricsFileUpnp *metric.MetricsFile
	var luaCollector *collector.Collector
	var upnpCollector *collector.Collector

	flag.Parse()

	// flagGatewayLuaURL
	u, err := url.Parse(*flagGatewayLuaURL)
	if err != nil {
		fmt.Println("invalid URL:", err)
		return
	}

	// init LuaCollector
	if *flagMetricsLuaFile != "" {
		err := readAndParseFile(*flagMetricsLuaFile, &metricsFileLua)
		if err != nil {
			fmt.Println(err)
			return
		}
		luaCollector, err = collector.NewLuaCollector(metricsFileLua, *flagGatewayLuaURL, *flagUsername, *flagPassword, u.Hostname())
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	// init UpnpCollector
	if *flagMetricsUpnpFile != "" {
		err := readAndParseFile(*flagMetricsUpnpFile, &metricsFileUpnp)
		if err != nil {
			fmt.Println(err)
			return
		}
		upnpCollector, err = collector.NewUpnpCollector(metricsFileUpnp, *flagGatewayUpnpURL, *flagUsername, *flagPassword, u.Hostname())
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	// test mode
	if *flagTest {
		if luaCollector != nil {
			luaCollector.Test()
		}
		if upnpCollector != nil {
			upnpCollector.Test()
		}
		return
	}
	// prometheus mode
	if luaCollector != nil {
		prometheus.MustRegister(luaCollector)
	}
	if upnpCollector != nil {
		prometheus.MustRegister(upnpCollector)
	}

	http.Handle("/metrics", promhttp.Handler())
	fmt.Printf("metrics available at http://%s/metrics\n", *flagAddress)
	log.Fatal(http.ListenAndServe(*flagAddress, nil))

}

func readAndParseFile(file string, v interface{}) error {
	jsonData, err := ioutil.ReadFile(file)
	if err != nil {
		return fmt.Errorf("error reading metric file: %v", err)
	}

	err = json.Unmarshal(jsonData, v)
	if err != nil {
		return fmt.Errorf("error parsing JSON: %v", err)
	}
	return nil
}
