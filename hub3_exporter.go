package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ChannelInfo struct {
	ID, Freq, Width, Mod, Interleave, Power, Annex, SNR int
	PreRSErrors, PostRSErrors                           int
}

var (
	showVersion   = flag.Bool("Version", false, "Print version number and exit")
	listenAddress = flag.String("web.listen-address", ":9463", "IP And Port to expose metrics on")
	modemIP       = flag.String("modemIP", "192.168.100.1", "IP address of modem")
	timeout       = flag.Duration("timeout", 5*time.Second, "Timeout for scrapes")
	metricsPath   = flag.String("web.telemetry-path", "/metrics", "Path to metrics")

	channels   = make(map[int]ChannelInfo)
	uschannels = make(map[int]ChannelInfo)
)

const (
	namespace = "hub3"
)

func init() {
	flag.Usage = func() {
		fmt.Println("Usage: hub3_exporter [ ... ] \nFlags:")
		flag.PrintDefaults()
	}
	prometheus.MustRegister(version.NewCollector("hub3_exporter"))
}

func start() {
	log.Infof("Starting Hub3 Exporter (Version: %s)", version.Info())

	exporter := ProExporter(*timeout)
	prometheus.MustRegister(exporter)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>Hub3 Exporter: Ver ` + version.Info() + `</title></head>` +
			`<body><h1>Virgin Media / UPC Hub3 Metrics exporter</h1>` +
			`<p><a href="` + *metricsPath + `">Metrics</a></p>` +
			`</body></html>`))
	})
	http.Handle(*metricsPath, promhttp.Handler())
	log.Infof("Metrics exposed at %s on %s", *metricsPath, *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))

}

func (p *PrometheusExporter) Collect(ch chan<- prometheus.Metric) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Make a HTTP client
	var httpClient = &http.Client{
		Timeout: time.Second * 30,
	}
	// Get Docsis Stats
	response, err := httpClient.Get(fmt.Sprintf("http://%s/walk?oids=1.3.6.1.2.1.10.127.1.1.1;", *modemIP))
	if err != nil {
		//LOG
		fmt.Println(err)
	}

	// Convert from JSONish response to something we can iterate
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	var x map[string]interface{}
	json.Unmarshal([]byte(body), &x)
	for key, value := range x {
		// Split the results on the last part of OID (index)
		oid := strings.Split(key, ".")
		// If oid > 2 start processing it
		if len(oid) > 2 {
			// Convert index to Int
			index, err := strconv.Atoi(oid[len(oid)-1])
			if err != nil {
				// Error and quit
			}
			// Convert metric to Int
			metric, err := strconv.Atoi(oid[len(oid)-2])
			if err != nil {
				//quit
			}
			c := channels[index]
			if c.ID == 0 {
				log.Debugln("Found new Channel", index)
				channels[index] = ChannelInfo{ID: index, Freq: 1, Width: 2, Mod: 3, Interleave: 4, Power: 5, Annex: 6,
					PreRSErrors: 7, PostRSErrors: 8}
			}
			value, _ := strconv.Atoi(value.(string))
			switch metric {
			case 1:
				c.ID = value
			case 2:
				c.Freq = value
			case 3:
				c.Width = value
			case 4:
				c.Mod = value
			case 5:
				c.Interleave = value
			case 6:
				c.Power = value
			case 7:
				c.Annex = value
			default:
			}
			channels[index] = c
		}
	}

	// Get Docsis Stats
	snr_resp, err := httpClient.Get(fmt.Sprintf("http://%s/walk?oids=1.3.6.1.4.1.4491.2.1.20.1.24.1.1;", *modemIP))
	if err != nil {
		//LOG
		fmt.Println(err)
	}
	// Convert from JSON to something we can iterate
	defer snr_resp.Body.Close()
	snrbody, err := ioutil.ReadAll(snr_resp.Body)
	var b map[string]interface{}
	json.Unmarshal([]byte(snrbody), &b)
	for key, value := range b {
		// Split the results on the last part of OID (index)
		oid := strings.Split(key, ".")
		// If oid > 2 start processing it
		if len(oid) > 2 {
			// Convert index to Int
			index, err := strconv.Atoi(oid[len(oid)-1])
			if err != nil {
				// Error and quit
			}
			c := channels[index]
			value, _ := strconv.Atoi(value.(string))
			c.SNR = value
			channels[index] = c
		}
	}

	// Get DS error stats
	ds_pre_rs, err := httpClient.Get(fmt.Sprintf("http://%s/walk?oids=1.3.6.1.2.1.10.127.1.1.4.1.3;", *modemIP))
	if err != nil {
		//LOG
		fmt.Println(err)
	}
	// Convert from JSON to something we can iterate
	defer ds_pre_rs.Body.Close()
	ds_pre_rs_body, err := ioutil.ReadAll(ds_pre_rs.Body)
	var a map[string]interface{}
	json.Unmarshal([]byte(ds_pre_rs_body), &a)
	for key, value := range a {
		// Split the results on the last part of OID (index)
		oid := strings.Split(key, ".")
		// If oid > 2 start processing it
		if len(oid) > 2 {
			// Convert index to Int
			index, err := strconv.Atoi(oid[len(oid)-1])
			if err != nil {
				// Error and quit
			}
			c := channels[index]
			value, _ := strconv.Atoi(value.(string))
			c.PreRSErrors = value
			channels[index] = c
		}
	}
	ds_post_rs, err := httpClient.Get(fmt.Sprintf("http://%s/walk?oids=1.3.6.1.2.1.10.127.1.1.4.1.3;", *modemIP))
	if err != nil {
		//LOG
		fmt.Println(err)
	}
	// Convert from JSON to something we can iterate
	defer ds_pre_rs.Body.Close()
	ds_post_rs_body, err := ioutil.ReadAll(ds_post_rs.Body)
	var y map[string]interface{}
	json.Unmarshal([]byte(ds_post_rs_body), &y)
	for key, value := range y {
		// Split the results on the last part of OID (index)
		oid := strings.Split(key, ".")
		// If oid > 2 start processing it
		if len(oid) > 2 {
			// Convert index to Int
			index, err := strconv.Atoi(oid[len(oid)-1])
			if err != nil {
				// Error and quit
			}
			c := channels[index]
			value, _ := strconv.Atoi(value.(string))
			c.PostRSErrors = value
			channels[index] = c
		}
	}
	up_resp, err := httpClient.Get(fmt.Sprintf("http://%s/walk?oids=1.3.6.1.4.1.4115.1.3.4.1.9.2;", *modemIP))
	if err != nil {
	}
	defer up_resp.Body.Close()
	upbody, err := ioutil.ReadAll(up_resp.Body)
	var z map[string]interface{}
	json.Unmarshal([]byte(upbody), &z)
	for key, value := range z {
		// Split the results on the last part of OID (index)
		oid := strings.Split(key, ".")
		// If oid > 2 start processing it
		if len(oid) > 2 {
			// Convert index to Int
			index, err := strconv.Atoi(oid[len(oid)-1])
			if err != nil {
				// Error and quit
			}
			// Convert metric to Int
			metric, err := strconv.Atoi(oid[len(oid)-2])
			if err != nil {
				//quit
			}
			c := uschannels[index]
			if c.ID == 0 {
				log.Debugln("Found new Channel", index)
				uschannels[index] = ChannelInfo{ID: index, Freq: 1, Width: 2, Mod: 3, Interleave: 4, Power: 5, Annex: 6}
			}
			value, _ := strconv.Atoi(value.(string))
			switch metric {
			case 1:
				c.ID = value
			default:
			}
			uschannels[index] = c
		}
	}

	usp_resp, err := httpClient.Get(fmt.Sprintf("http://%s/walk?oids=1.3.6.1.4.1.4491.2.1.20.1.2.1.1;", *modemIP))
	if err != nil {
		//LOG
		fmt.Println(err)
	}
	// Convert from JSON to something we can iterate
	defer usp_resp.Body.Close()
	uspbody, err := ioutil.ReadAll(usp_resp.Body)
	var w map[string]interface{}
	json.Unmarshal([]byte(uspbody), &w)
	for key, value := range w {
		// Split the results on the last part of OID (index)
		oid := strings.Split(key, ".")
		// If oid > 2 start processing it
		if len(oid) > 2 {
			// Convert index to Int
			index, err := strconv.Atoi(oid[len(oid)-1])
			if err != nil {
				// Error and quit
			}
			c := uschannels[index]
			value, _ := strconv.Atoi(value.(string))
			c.Power = value
			uschannels[index] = c
		}
	}

	for _, c := range channels {
		ch <- prometheus.MustNewConstMetric(p.downFrequency, prometheus.GaugeValue, float64(c.Freq), strconv.Itoa(c.ID))
		ch <- prometheus.MustNewConstMetric(p.downPower, prometheus.GaugeValue, float64(c.Power)/10, strconv.Itoa(c.ID))
		ch <- prometheus.MustNewConstMetric(p.downSNR, prometheus.GaugeValue, float64(c.SNR)/10, strconv.Itoa(c.ID))
		ch <- prometheus.MustNewConstMetric(p.downPreRS, prometheus.GaugeValue, float64(c.PreRSErrors), strconv.Itoa(c.ID))
		ch <- prometheus.MustNewConstMetric(p.downPostRS, prometheus.GaugeValue, float64(c.PostRSErrors), strconv.Itoa(c.ID))
	}
	for _, c := range uschannels {
		// Frequency needs another grab from http://192.168.100.1/walk?oids=1.3.6.1.2.1.10.127.1.1.2;&_n=86936&_=1507125744036
		ch <- prometheus.MustNewConstMetric(p.upPower, prometheus.GaugeValue, float64(c.Power)/10, strconv.Itoa(c.ID))
	}
}

type PrometheusExporter struct {
	mutex sync.Mutex

	downFrequency *prometheus.Desc
	downPower     *prometheus.Desc
	downSNR       *prometheus.Desc
	downPreRS     *prometheus.Desc
	downPostRS    *prometheus.Desc
	upFrequency   *prometheus.Desc
	upPower       *prometheus.Desc
}

func ProExporter(timeout time.Duration) *PrometheusExporter {
	return &PrometheusExporter{
		downFrequency: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "downstream", "frequency_hertz"),
			"Downstream Frequency in HZ",
			[]string{"channel"},
			nil,
		),
		downPower: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "downstream", "power_dbmv"),
			"Downstream Power level in dBmv",
			[]string{"channel"},
			nil,
		),
		downSNR: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "downstream", "snr_db"),
			"Downstream SNR in dB",
			[]string{"channel"},
			nil,
		),
		downPostRS: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "downstream", "post_rs_errors"),
			"Number of Errors per channel Post RS",
			[]string{"channel"},
			nil,
		),
		downPreRS: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "downstream", "pre_rs_errors"),
			"Number of Errors per channel Pre RS",
			[]string{"channel"},
			nil,
		),
		upFrequency: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "upstream", "frequency_hertz"),
			"Upstream Frequency in HZ",
			[]string{"channel"},
			nil,
		),
		upPower: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "upstream", "power_dbmv"),
			"Upstream Power level in dBmv",
			[]string{"channel"},
			nil,
		),
	}
}

func (p *PrometheusExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- p.downFrequency
	ch <- p.upFrequency
	ch <- p.downPower
	ch <- p.upPower
	ch <- p.downSNR
	ch <- p.downPostRS
	ch <- p.downPreRS
}

func main() {
	flag.Parse()
	if *showVersion {
		os.Exit(0)
	}
	start()
}
