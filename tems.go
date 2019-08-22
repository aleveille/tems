package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/aleveille/tems/check"
	"github.com/aleveille/tems/config"
	"github.com/aleveille/tems/dataout"
	"github.com/aleveille/tems/datasource"
	log "github.com/aleveille/tems/logger"
)

func init() {
	err := log.InitLogger()

	if err != nil {
		fmt.Printf("error while setting up the logging: \n%s", err)
		os.Exit(1)
	}
}

func main() {
	log.Println("TSDB performance evaluation program start")

	var err error

	err = config.InitConfigFromEnvVars()
	if err != nil {
		log.Fatal(err)
	}

	err = parseCLIFlag()
	if err != nil {
		log.Fatal(err)
	}

	err = config.ValidateConfig()
	if err != nil {
		log.Fatal(err)
	}

	err = dataout.InitResultChan()
	if err != nil {
		log.Fatal(err)
	}

	_, err = datasource.InitAWSProxy()
	if err != nil {
		log.Fatal(err)
	}

	_, err = datasource.InitGrafanaProxy()
	if err != nil {
		log.Fatal(err)
	}

	_, err = dataout.InitCirconusProxy()
	if err != nil {
		log.Fatal(err)
	}

	switch config.TSDBSystem {
	case "irondb":
		check.EvaluateIRONdb()
	case "influxdb":
		check.EvaluateInfluxDB()
	case "timescale":
		check.EvaluateTimescaleDB()
	default:
		log.Fatal("Config validation should have caught that the TSDB type is invalid")
	}
}

func parseCLIFlag() error {
	var sandboxID string
	var tsdbSystem string
	var circonusAPIToken string
	var grafanaURL string
	var grafanaUser string
	var grafanaPassword string
	var awsProfile string
	var awsRegion string
	var awsExpectedASGs int
	var awsExpectedInstanceCountPerASG int
	var caqlUseTags bool
	var logLevel string

	flag.StringVar(&sandboxID, "sandboxID", "", "Something like irondb-sandbox")
	flag.StringVar(&tsdbSystem, "tsdbSystem", "", "Lowercase TSDB system type (irondb, influxdb, timescale, etc)")
	flag.StringVar(&circonusAPIToken, "circonusAPIToken", "", "A Circonus API token (normal privileges are enough)")
	flag.StringVar(&grafanaURL, "grafanaURL", "", "Something like https://grafana.<sandbox-id>.adgear-dev.com")
	flag.StringVar(&grafanaUser, "grafanaUser", "", "The user used to login to Grafana")
	flag.StringVar(&grafanaPassword, "grafanaPassword", "", "The password used to login to Grafana")
	flag.StringVar(&awsProfile, "awsProfile", "", "The AWS profile to use for auth")
	flag.StringVar(&awsRegion, "awsRegion", "", "The AWS region to query")
	flag.IntVar(&awsExpectedASGs, "awsExpectedASGs", -1, "The number of ASGs expected for this TSDB configuration")
	flag.IntVar(&awsExpectedInstanceCountPerASG, "awsExpectedInstanceCountPerASG", -1, "The expected number of instances in each ASG")
	flag.BoolVar(&caqlUseTags, "irondbCaqlUseTags", false, "Whether to use the tag version of the CAQL queries")
	flag.StringVar(&logLevel, "logLevel", "", "Log level")

	flag.Parse()

	if sandboxID != "" {
		config.SandboxID = sandboxID
	}

	if tsdbSystem != "" {
		config.TSDBSystem = tsdbSystem
	}

	if circonusAPIToken != "" {
		config.CirconusAPIToken = circonusAPIToken
	}

	if grafanaURL != "" {
		config.GrafanaURL = grafanaURL
	}

	if grafanaUser != "" {
		config.GrafanaUser = grafanaUser
	}

	if grafanaPassword != "" {
		config.GrafanaPassword = grafanaPassword
	}

	if awsProfile != "" {
		config.AWSProfile = awsProfile
	}

	if awsRegion != "" {
		config.AWSRegion = awsRegion
	}

	if awsExpectedASGs != -1 {
		config.AWSExpectedASGs = awsExpectedASGs
	}

	if awsExpectedInstanceCountPerASG != -1 {
		config.AWSExpectedInstanceCountPerASG = awsExpectedInstanceCountPerASG
	}

	if caqlUseTags != false {
		config.CAQLUseTags = caqlUseTags
	}

	if logLevel != "" {
		config.LogLevel = logLevel
	}

	return nil
}
