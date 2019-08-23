package dataout

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/aleveille/tems/config"
	appError "github.com/aleveille/tems/error"
	log "github.com/aleveille/tems/logger"
	circonusApi "github.com/circonus-labs/go-apiclient"
)

var (
	// CirconusProxyInstance is the globally accessible CirconusProxy struct
	CirconusProxyInstance CirconusProxy
	// TODO put all metrics in there?: asg.irondb-nodes-sidea.cpu.utilization.avg, node.node-1.disk.write.bytes, eg
	queryMetricPrefixes = []string{"query"}
	queryMetrics        = []string{
		"metrics-count",
		"1-ts-24-hour-range",
		"1-ts-1-week-range",
		"100-ts-1-week-range",
		"400-ts-1-week-range",
		"100-ts-6-hour-range",
		"400-ts-6-hour-range",
		"100-ts-p99-1-week-range",
		"400-ts-p99-1-week-range",
		"100-ts-mean-1-week-range",
		"400-ts-mean-1-week-range",
	}
	queryMetricSuffixes = []string{"duration", "value"}

	infraAsgMetricPrefixes  = []string{"infra.tsdb-asg-"}
	infraNodeMetricPrefixes = []string{"infra.tsdb-node-"}
	infraMetrics            = []string{
		"cpu.utilization.avg",
		"network.in.bytes",
		"network.out.bytes",
		"disk.read.bytes",
		"disk.write.bytes",
		"ebs.read.bytes",
		"ebs.write.bytes",
	}
)

// CirconusProxy is our wrapper to provide higher-level functionnality to the Circonus API
// It maintains its API session and will create JSON API requests for operations not covered by the SDK
type CirconusProxy struct {
	apiClient     *circonusApi.API
	httpAPIclient *http.Client
	httpAPIURL    string
	checkBundleID string
}

// InitCirconusProxy initialize the CirconusProxy struct in order to interact with Circonus' SaaS
func InitCirconusProxy() (*CirconusProxy, error) {
	log.Debug("InitCirconusProxy() start")
	defer log.Debug("InitCirconusProxy() end")

	proxy := CirconusProxy{}

	circonusAPIclient, err := circonusApi.New(&circonusApi.Config{TokenKey: config.CirconusAPIToken})
	if err != nil {
		return &proxy, appError.NewInitializationError("error while initializing the Circonus API client", err)
	}
	proxy.apiClient = circonusAPIclient

	circonusHTTPAPIclient := circonusHTTPAPIclientSetup()
	proxy.httpAPIclient = circonusHTTPAPIclient

	err = proxy.apiClientCheck()
	if err != nil {
		return &proxy, appError.NewInitializationError("error while validating that the Circonus API client can read data", err)
	}

	err = proxy.createSandboxBundle()
	if err != nil {
		return &proxy, appError.NewInitializationError("error while creating check metric bundle", err)
	}

	CirconusProxyInstance = proxy
	return &proxy, nil
}

func circonusHTTPAPIclientSetup() *http.Client {
	httpTransport := &http.Transport{
		DisableCompression: true,
		MaxConnsPerHost:    0,
		Dial: (&net.Dialer{
			Timeout: 200 * time.Millisecond,
		}).Dial,
		IdleConnTimeout:       200 * time.Millisecond,
		ResponseHeaderTimeout: 500 * time.Millisecond,
		TLSHandshakeTimeout:   500 * time.Millisecond,
	}

	circonusHTTPAPIclient := &http.Client{
		Timeout:   1 * time.Second,
		Transport: httpTransport,
	}

	return circonusHTTPAPIclient
}

func (c *CirconusProxy) apiClientCheck() error {
	log.Trace("Circonus apiClientCheck() start")
	defer log.Trace("Circonus apiClientCheck() end")

	user, err := c.apiClient.FetchUser(nil)
	if err != nil || user == nil {
		return err
	}

	return nil
}

func (c *CirconusProxy) createSandboxBundle() error {
	log.Trace("Circonus createSandboxBundle() start")
	defer log.Trace("Circonus createSandboxBundle() end")

	stringSearchQuery := fmt.Sprintf("(type:httptrap)(display_name:\"httptrap - %s\")", config.SandboxID)
	searchQuery := circonusApi.SearchQueryType(stringSearchQuery)

	cBundles, err := c.apiClient.SearchCheckBundles(&searchQuery, nil)
	if err != nil {
		return appError.NewInitializationError("Error while searching for bundles", err)
	}

	foundBundles := len(*cBundles)
	log.Debugf("Found %d bundles", foundBundles)

	if foundBundles >= 2 {
		return appError.NewInitializationError(fmt.Sprintf("Error while looking for Circonus Check Bundles. Found %d bundles, but was expecting only one", foundBundles), nil)
	} else if foundBundles == 1 {
		cBundle := (*cBundles)[0]
		c.httpAPIURL = cBundle.Config["submission_url"]
		c.checkBundleID = cBundle.CID
	} else {
		// create it
		cBundle := circonusApi.NewCheckBundle()
		cBundle.Brokers = []string{"/broker/35"}
		cBundle.Timeout = 10
		cBundle.Status = "active"
		cBundle.Target = fmt.Sprintf("tems/fakepath/%s", config.SandboxID)
		cBundle.DisplayName = fmt.Sprintf("httptrap - %s", config.SandboxID)
		cBundle.Config["secret"] = "mys3cr3t"
		cBundle.Config["asynch_metrics"] = "false"
		cBundle.Tags = []string{fmt.Sprintf("sandbox:%s", config.SandboxID)}
		cBundle.Type = "httptrap"

		metrics, _ := c.createMetricsArray()
		cBundle.Metrics = metrics

		updatedCBundle, err := c.apiClient.CreateCheckBundle(cBundle)
		if err != nil {
			return appError.NewInitializationError("Error while saving check bundle", err)
		}

		c.httpAPIURL = updatedCBundle.Config["submission_url"]
		c.checkBundleID = updatedCBundle.CID
	}

	return nil
}

func (c *CirconusProxy) createMetricsArray() ([]circonusApi.CheckBundleMetric, error) {
	log.Trace("Circonus createAllMetrics() start (this takes about 2 minutes)")
	defer log.Trace("Circonus createAllMetrics() end")

	cBundleMetricArr := make([]circonusApi.CheckBundleMetric, len(queryMetrics)*2+len(infraMetrics)*(config.AWSExpectedASGs+config.AWSExpectedASGs*config.AWSExpectedInstanceCountPerASG))
	metricCount := 0

	log.Tracef("Created a metric array %d wide", len(cBundleMetricArr))

	tags := []string{"sandbox: %s", "category: query", "source: grafana"}
	for _, prefix := range queryMetricPrefixes {
		for _, metricName := range queryMetrics {
			for _, suffix := range queryMetricSuffixes {
				metricFullname := fmt.Sprintf("%s.%s.%s.%s", config.SandboxID, prefix, metricName, suffix)

				cBundleMetricArr[metricCount] = *c.createMetric(metricFullname, tags)
				metricCount++
			}
		}
	}
	log.Tracef("Query metrics done, %d metrics created so far", metricCount)

	for _, prefix := range infraAsgMetricPrefixes {
		for i := 1; i <= config.AWSExpectedASGs; i++ {
			for _, metricName := range infraMetrics {
				metricFullname := fmt.Sprintf("%s.%s%d.%s", config.SandboxID, prefix, i, metricName)

				if strings.Contains(metricName, ".avg") {
					tags = []string{fmt.Sprintf("sandbox: %s", config.SandboxID), "category: asg", "source: aws", "aggregation: avg"}
				} else if strings.Contains(metricName, ".max") {
					tags = []string{fmt.Sprintf("sandbox: %s", config.SandboxID), "category: asg", "source: aws", "aggregation: max"}
				} else {
					tags = []string{fmt.Sprintf("sandbox: %s", config.SandboxID), "category: asg", "source: aws", "aggregation: sum"}
				}

				cBundleMetricArr[metricCount] = *c.createMetric(metricFullname, tags)
				metricCount++
			}
		}
	}
	log.Tracef("ASG metrics done, %d metrics created so far", metricCount)

	for _, prefix := range infraNodeMetricPrefixes {
		for i := 1; i <= config.AWSExpectedASGs*config.AWSExpectedInstanceCountPerASG; i++ {
			for _, metricName := range infraMetrics {
				metricFullname := fmt.Sprintf("%s.%s%d.%s", config.SandboxID, prefix, i, metricName)

				if strings.Contains(metricName, ".avg") {
					tags = []string{fmt.Sprintf("sandbox: %s", config.SandboxID), "category: asg", "source: aws", "aggregation: avg"}
				} else if strings.Contains(metricName, ".max") {
					tags = []string{fmt.Sprintf("sandbox: %s", config.SandboxID), "category: asg", "source: aws", "aggregation: max"}
				} else {
					tags = []string{fmt.Sprintf("sandbox: %s", config.SandboxID), "category: asg", "source: aws", "aggregation: sum"}
				}

				cBundleMetricArr[metricCount] = *c.createMetric(metricFullname, tags)
				metricCount++
			}
		}
	}
	log.Tracef("Node metrics done, %d metrics created so far", metricCount)

	return cBundleMetricArr, nil
}

func (c *CirconusProxy) createMetric(name string, tags []string) *circonusApi.CheckBundleMetric {
	if tags == nil {
		tags = []string{}
	}

	cbm := circonusApi.CheckBundleMetric{
		Name:   name,
		Status: "active",
		Tags:   tags,
		Type:   "numeric",
	}

	unitTypeNone := "none"
	unitTypePercent := "percent"
	unitTypeMilliseconds := "milliseconds"
	if strings.Contains(name, "duration") {
		cbm.Units = &unitTypeMilliseconds
	} else if strings.Contains(name, "cpu") {
		cbm.Units = &unitTypePercent
	} else {
		cbm.Units = &unitTypeNone
	}

	return &cbm
}

// PushDatapoint will push a datapoint (a value) to the Circonus SaaS platform
func (c *CirconusProxy) PushDatapoint(timestamp int64, name string, value string, datatype string) {
	var sb strings.Builder
	payload := fmt.Sprintf("{\"_ts\": %d, \"%s\": \"%s\", \"_type\": \"%s\"}", timestamp*1000, name, value, datatype)
	sb.WriteString(payload)

	req, _ := http.NewRequest("POST", c.httpAPIURL, strings.NewReader(sb.String()))
	req.Header.Add("X-Circonus-Auth-Token", config.CirconusAPIToken)
	req.Header.Add("X-Circonus-App-Name", "tems")
	_, err := c.httpAPIclient.Do(req)
	if err != nil {
		log.Errorf("error while sending the HTTP POST request:\n\t%s\n\tFor request payload %s\n", err, payload)
	}

}
