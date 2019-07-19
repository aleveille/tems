package datasource

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aleveille/tems/config"
	"github.com/aleveille/tems/dataout"
	appError "github.com/aleveille/tems/error"
	log "github.com/aleveille/tems/logger"
)

var (
	// GrafanaProxyInstance is the globally accessible GrafanaProxy struct
	GrafanaProxyInstance GrafanaProxy
	grafanaCookie        = "tdb"
	cookieRegexp         = regexp.MustCompile("(grafana_session=[^;]*).*Max-Age=([0-9]*)")

	// IRONdb (CAQL) specific variables:
	caqlQueryURL = "%s/api/datasources/proxy/1/extension/lua/caql_v1?format=DF4&start=%d&end=%d&period=60&q=%s"
	// Response body ~= "data":[[6000]],"meta"....
	// Match everything from the double [[ until a ]
	caqlResultRegex       = regexp.MustCompile("data\":\\[\\[([^\\]]*)")
	influxLastResultRegex = regexp.MustCompile(".*,([0-9]*)]]")

	// InfluxDB specific variables:
	influxdbQueryURL = "%s/api/datasources/proxy/1/query?db=%s&q=%s%%20&epoch=%s"
)

// GrafanaProxy is our wrapper to provide higher-level functionnality to the Grafana API
// It maintains its HTTP session (through a cookie) and will create JSON API requests to the Grafana API
type GrafanaProxy struct {
	httpAPIclient *http.Client
}

// InitGrafanaProxy initialize the GrafanaProxy struct in order to interact with Grafana API
func InitGrafanaProxy() (*GrafanaProxy, error) {
	log.Debug("InitGrafanaProxy() start")
	defer log.Debug("InitGrafanaProxy() end")
	proxy := GrafanaProxy{}

	grafanaHTTPAPIclient := grafanaHTTPAPIclientSetup()
	proxy.httpAPIclient = grafanaHTTPAPIclient

	err := proxy.login()
	if err != nil {
		return &proxy, appError.NewInitializationError("Error while logging in to Grafana", err)
	}

	GrafanaProxyInstance = proxy
	return &proxy, nil
}

func grafanaHTTPAPIclientSetup() *http.Client {
	httpTransport := &http.Transport{
		DisableCompression: true,
		MaxConnsPerHost:    0,
		Dial: (&net.Dialer{
			Timeout: 500 * time.Millisecond,
		}).Dial,
		IdleConnTimeout:       500 * time.Millisecond,
		ResponseHeaderTimeout: 1000 * time.Millisecond,
		TLSHandshakeTimeout:   1000 * time.Millisecond,
	}

	grafanaHTTPAPIclient := &http.Client{
		Timeout:   20 * time.Second,
		Transport: httpTransport,
	}

	return grafanaHTTPAPIclient
}

func (g *GrafanaProxy) login() error {
	log.Trace("Grafana login() start")
	defer log.Trace("Grafana login() end")
	loginURL := fmt.Sprintf("%s/login", config.GrafanaURL)

	payload := []byte(fmt.Sprintf(`{"user":"%s","email":"","password":"%s"}`, config.GrafanaUser, config.GrafanaPassword))
	req, err := http.NewRequest("POST", loginURL, bytes.NewBuffer(payload))
	if err != nil {
		return appError.NewInitializationError("error creating the HTTP request", err)
	}
	req.Header.Set("Content-Type", "application/json")

	response, err := g.httpAPIclient.Do(req)
	if response != nil {
		defer response.Body.Close()
	}

	if err != nil {
		return appError.NewInitializationError("error while sending the login request", err)
	}
	if response.StatusCode >= 400 {
		return appError.NewInitializationError(fmt.Sprintf("error while sending the login request. HTTP status: %d", response.StatusCode), nil)
	}

	// TODO: This part seems brittle using hardcoded array index access (response.Header["Set-Cookie"][0], grafanaCookie = match[1] & strconv.Atoi(match[2])
	// It's probably possible to refactor this to something cleaner
	// Also todo: break the parsing of the response headers into another func
	if response.Header["Set-Cookie"] != nil {
		headerCookie := response.Header["Set-Cookie"][0]
		match := cookieRegexp.FindStringSubmatch(headerCookie)

		if len(match) < 3 {
			return appError.NewInitializationError(fmt.Sprintf("unexpected match length for login cookie. response.Header[\"Set-Cookie\"]=%s", response.Header["Set-Cookie"]), nil)
		}

		grafanaCookie = match[1]

		maxage, parseErr := strconv.Atoi(match[2])
		if parseErr != nil || maxage < 5 {
			maxage = 5 // retry in 5s
			log.Warn("Grafana: Unexpected maxage value in the cookie response header. Defaulting to 5s.")
		}

		log.Debugf("Grafana: Successfully logged in. Cookie valid for %d seconds", maxage)

		relogTimer := time.NewTimer(time.Duration(maxage) * time.Second)
		go func() {
			<-relogTimer.C
			g.login()
		}()
	}
	return nil
}

// TODO: Review if this needs refactoring (spoiler: it does)
func (g *GrafanaProxy) SimpleCaqlQuery(queryMetricName string, caqlQuery string, queryRange int64) {
	queryStartTime := time.Now()

	queryDuration := "nan"

	queryTimestamp := queryStartTime.Unix()
	result, err := g.doProxiedCAQLHTTPQuery(caqlQuery, queryTimestamp-queryRange, queryTimestamp)

	if err != nil {
		result = "nan"
		log.Errorf("Error while querying Grafana:\n%v\n", err)
	} else {
		queryDurationFloat := float64(time.Now().UnixNano()-queryStartTime.UnixNano()) / 1000 / 1000
		queryDuration = fmt.Sprintf("%.2f", queryDurationFloat)
	}

	select {
	case dataout.ResultChan <- dataout.Result{Timestamp: queryTimestamp, Name: fmt.Sprintf("%s.query.%s.duration", config.SandboxID, queryMetricName), Value: queryDuration}:
	default:
		log.Error("Channel full, discarding result")
	}
	select {
	case dataout.ResultChan <- dataout.Result{Timestamp: queryTimestamp, Name: fmt.Sprintf("%s.query.%s.value", config.SandboxID, queryMetricName), Value: result}:
	default:
		log.Error("Channel full, discarding result")
	}
}

// TODO: Review if this needs refactoring (spoiler: it does)
func (g *GrafanaProxy) doProxiedCAQLHTTPQuery(queryString string, startTimestamp int64, endTimestamp int64) (string, error) {
	var netClient = &http.Client{
		Timeout: time.Second * 25,
	}

	formattedCaqlURL := fmt.Sprintf(caqlQueryURL, config.GrafanaURL, startTimestamp, endTimestamp, queryString)
	req, _ := http.NewRequest("GET", formattedCaqlURL, nil)
	req.Header.Add("cookie", grafanaCookie)
	req.Header.Add("x-circonus-account", "1")

	response, err := netClient.Do(req)
	if response != nil {
		defer response.Body.Close()
	}

	if err != nil {
		return "nan", fmt.Errorf("net/client request error while querying the Grafana CAQL proxy:\n\t%s", err)
	}
	if response.StatusCode >= 400 {
		return "nan", fmt.Errorf("unexpected HTTP status code error while querying the Grafana CAQL proxy. HTTP status: %d", response.StatusCode)
	}

	body, ioErr := ioutil.ReadAll(response.Body)
	if ioErr != nil {
		return "nan", fmt.Errorf("io error while reading HTTP response body:\n%s", ioErr)
	}

	stringBody := string(body)
	match := caqlResultRegex.FindStringSubmatch(stringBody)

	if len(match) < 2 {
		return "nan", nil
	}

	datapoints := strings.Split(match[1], ",")
	lastCount := datapoints[len(datapoints)-1]
	return lastCount, nil
}

func (g *GrafanaProxy) SimpleInfluxDBQuery(queryMetricName string, queryString string, queryRange string) {
	queryStartTime := time.Now()

	queryDuration := "nan"

	queryTimestamp := queryStartTime.Unix()
	result, err := g.doProxiedInfluxDBHTTPQuery(config.InfluxDBDatabaseName, queryString, queryRange, config.InfluxDBEpoch)

	if err != nil {
		result = "nan"
		log.Errorf("Error while querying Grafana:\n%v\n", err)
	} else {
		queryDurationFloat := float64(time.Now().UnixNano()-queryStartTime.UnixNano()) / 1000 / 1000
		queryDuration = fmt.Sprintf("%.2f", queryDurationFloat)
	}

	select {
	case dataout.ResultChan <- dataout.Result{Timestamp: queryTimestamp, Name: fmt.Sprintf("%s.query.%s.duration", config.SandboxID, queryMetricName), Value: queryDuration}:
	default:
		log.Error("Channel full, discarding result")
	}
	select {
	case dataout.ResultChan <- dataout.Result{Timestamp: queryTimestamp, Name: fmt.Sprintf("%s.query.%s.value", config.SandboxID, queryMetricName), Value: result}:
	default:
		log.Error("Channel full, discarding result")
	}
}

func (g *GrafanaProxy) doProxiedInfluxDBHTTPQuery(db string, queryString string, queryRange string, epoch string) (string, error) {
	var netClient = &http.Client{
		Timeout: time.Second * 25,
	}

	rangeQuery := fmt.Sprintf(queryString, queryRange)
	formattedQueryURL := fmt.Sprintf(influxdbQueryURL, config.GrafanaURL, db, url.PathEscape(rangeQuery), epoch)

	req, _ := http.NewRequest("GET", formattedQueryURL, nil)
	req.Header.Add("cookie", grafanaCookie)

	log.Tracef("Request sent to Grafana: URL=%v, Cookies=%v", req.URL, req.Cookies())

	response, err := netClient.Do(req)
	if response != nil {
		defer response.Body.Close()
	}

	if err != nil {
		return "nan", fmt.Errorf("net/client request error while querying the Grafana InfluxDB proxy:\n\t%s", err)
	}
	if response.StatusCode >= 400 {
		return "nan", fmt.Errorf("unexpected HTTP status code error while querying the Grafana InfluxDB proxy. HTTP status: %d", response.StatusCode)
	}

	body, ioErr := ioutil.ReadAll(response.Body)
	if ioErr != nil {
		return "nan", fmt.Errorf("io error while reading HTTP response body:\n%s", ioErr)
	}

	stringBody := string(body)
	match := influxLastResultRegex.FindStringSubmatch(stringBody)

	if len(match) < 2 {
		return "nan", nil
	}

	datapoints := strings.Split(match[1], ",")
	lastCount := datapoints[len(datapoints)-1]
	return lastCount, nil
}
