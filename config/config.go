package config

import (
	"os"
	"strconv"

	"github.com/sirupsen/logrus"

	appError "github.com/aleveille/tems/error"
	log "github.com/aleveille/tems/logger"
)

var (
	// SandboxID is the string identifier of the sandbox test (eg: irondb-tags)
	SandboxID string

	// TSDBSystem is type of TSDB being evaluated (IRONdb, InfluxDB, etc)
	TSDBSystem = "irondb"

	// CirconusAPIToken is the API token used to talk to Circonus SaaS API
	CirconusAPIToken = ""

	// GrafanaURL is the URL of the Grafana configured for the sandbox
	GrafanaURL string

	// GrafanaUser is the user used to login to Grafana
	GrafanaUser = "admin"

	// GrafanaPassword is the password used to login to Grafana
	GrafanaPassword = ""

	// AWSProfile is the profile to be used by the AWS SDK when calling the AWS API
	AWSProfile string

	// AWSRegion is the region (us-east-1, etc) to be used by the AWS SDK when calling the AWS API
	AWSRegion = "us-east-1"

	// AWSExpectedASGs is the number of ASGs expected for this TSDB configuration (eg: 2 ASG for a two-sided IRONdb configuration)
	AWSExpectedASGs = 2

	// AWSExpectedInstanceCountPerASG is the expected number of instances in each ASG. (eg: 3 instances per ASG for a six nodes IRONdb cluster)
	AWSExpectedInstanceCountPerASG = 3

	// CAQLUseTags is whether the IRONdb CAQL queries should use tags (otherwise they'll use namespacing)
	CAQLUseTags = false

	// InfluxDBDatabaseName is the name of the DB to query
	InfluxDBDatabaseName = "mydb"

	// InfluxDBEpoch is the epoch parameter when sending proxied InfluxDB queries
	InfluxDBEpoch = "ms"

	// LogLevel is the logrus log level
	LogLevel string
)

// InitConfigFromEnvVars will set some config variables from their environment variables equivalent
// This should be called before parsing CLI flags (in other words, CLI flags should overwrite env var values)
func InitConfigFromEnvVars() error {
	var val string

	val = os.Getenv("SANDBOX_ID")
	if val != "" {
		SandboxID = val
	}

	val = os.Getenv("TSDB_SYSTEM")
	if val != "" {
		TSDBSystem = val
	}

	val = os.Getenv("CIRCONUS_API_TOKEN")
	if val != "" {
		CirconusAPIToken = val
	}

	val = os.Getenv("GRAFANA_URL")
	if val != "" {
		GrafanaURL = val
	}

	val = os.Getenv("GRAFANA_USER")
	if val != "" {
		GrafanaUser = val
	}

	val = os.Getenv("GRAFANA_PASSWORD")
	if val != "" {
		GrafanaPassword = val
	}

	val = os.Getenv("AWS_PROFILE")
	if val != "" {
		AWSProfile = val
	}

	val = os.Getenv("AWS_REGION")
	if val != "" {
		AWSRegion = val
	}

	val = os.Getenv("AWS_EXPECTED_ASGS")
	if val != "" {
		ival, err := strconv.Atoi(val)
		if err != nil {
			return appError.NewInitializationError("Error parsing integer value for AWS_EXPECTED_ASGS", err)
		}

		AWSExpectedASGs = ival
	}

	val = os.Getenv("AWS_EXPECTED_INSTANCE_COUNT_PER_ASG")
	if val != "" {
		ival, err := strconv.Atoi(val)
		if err != nil {
			return appError.NewInitializationError("Error parsing integer value for AWS_EXPECTED_INSTANCE_COUNT_PER_ASG", err)
		}

		AWSExpectedInstanceCountPerASG = ival
	}

	val = os.Getenv("CAQL_USE_TAGS")
	if val != "" {
		bval, err := strconv.ParseBool(val)
		if err != nil {
			return appError.NewInitializationError("Error parsing boolean value for CAQL_USE_TAGS", err)
		}

		CAQLUseTags = bval
	}

	val = os.Getenv("LOG_LEVEL")
	if val != "" {
		LogLevel = val
	}

	return nil
}

// ValidateConfig will validate the program configuration variables values. This should be called after InitConfigFromEnvVars() and flag.Parse()
func ValidateConfig() error {
	if SandboxID == "" {
		return appError.NewInitializationError("The variable sandboxID must be provided through the CLI arguments or environment variable", nil)
	}

	if GrafanaURL == "" {
		return appError.NewInitializationError("The variable grafanaURL must be provided through the CLI arguments or environment variable", nil)
	}

	if GrafanaPassword == "" {
		return appError.NewInitializationError("The variable grafanaPassword must be provided through the CLI arguments or environment variable", nil)
	}

	if CirconusAPIToken == "" {
		return appError.NewInitializationError("The variable circonusAPIToken must be provided through the CLI arguments or environment variable", nil)
	}

	if TSDBSystem != "irondb" && TSDBSystem != "influxdb" {
		return appError.NewInitializationError("The value of tsdbSystem is invalid", nil)
	}

	logrusLevel, err := logrus.ParseLevel(LogLevel)
	if err != nil {
		return appError.NewInitializationError("Error parsing log level value for LOG_LEVEL", err)
	}
	log.SetLevel(logrusLevel)

	log.Debug("Configuration validated successfully")

	return nil
}
