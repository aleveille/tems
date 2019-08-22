package check

import (
	"fmt"
	"time"

	"github.com/aleveille/tems/config"
	"github.com/aleveille/tems/datasource"

	log "github.com/aleveille/tems/logger"
)

// 6H  intervalMs	60000
// 24H intervalMs	120000
// 1W  intervalMs   600000

var (
	timescaleQueryMetricCount   = `SELECT $__timeGroupAlias(\"time\",$__interval), count(value) AS \"value\" FROM \"randomint1\" WHERE $__timeFilter(\"time\") AND worker = '1' GROUP BY time ORDER BY time`
	timescaleQuery1Timeserie    = `SELECT $__timeGroupAlias(\"time\",$__interval), sum(value) AS \"value\", worker AS \"metric\" FROM \"randomint1\" WHERE $__timeFilter(\"time\") AND worker = '1' GROUP BY worker, time ORDER BY time`
	timescaleQuery100Timeseries = `SELECT $__timeGroupAlias(\"time\",$__interval), sum(value) AS \"value\", worker AS \"metric\" FROM \"randomint1\" WHERE $__timeFilter(\"time\") AND worker SIMILAR TO '1[0-9][0-9]' GROUP BY worker, time ORDER BY time`
	timescaleQuery400Timeseries = `SELECT $__timeGroupAlias(\"time\",$__interval), sum(value) AS \"value\", worker AS \"metric\" FROM \"randomint1\" WHERE $__timeFilter(\"time\") AND worker SIMILAR TO '[1-4][0-9][0-9]' GROUP BY worker, time ORDER BY time`
	//find("lagrande.latency.lg1.*")|Stats:percentile(99)
	timescaleQuery100TimeseriesP99  = `SELECT $__timeGroupAlias(\"time\",$__interval), sum(value) AS \"value\" FROM \"randomint1\" WHERE $__timeFilter(\"time\") AND worker SIMILAR TO '1[0-9][0-9]' GROUP BY time ORDER BY time`
	timescaleQuery400TimeseriesP99  = `SELECT $__timeGroupAlias(\"time\",$__interval), sum(value) AS \"value\" FROM \"randomint1\" WHERE $__timeFilter(\"time\") AND worker SIMILAR TO '[1-4][0-9][0-9]' GROUP BY time ORDER BY time`
	timescaleQuery100TimeseriesMean = `SELECT $__timeGroupAlias(\"time\",$__interval), percentile_cont(0.95) WITHIN GROUP (ORDER BY value) FROM \"randomint1\" WHERE $__timeFilter(\"time\") AND worker SIMILAR TO '1[0-9][0-9]' GROUP BY time ORDER BY time`
	timescaleQuery400TimeseriesMean = `SELECT $__timeGroupAlias(\"time\",$__interval), percentile_cont(0.95) WITHIN GROUP (ORDER BY value) FROM \"randomint1\" WHERE $__timeFilter(\"time\") AND worker SIMILAR TO '[1-4][0-9][0-9]' GROUP BY time ORDER BY time`
)

// EvaluateIRONdb will launch the AWS and timescale queries to assess IRONdb performances
func EvaluateTimescaleDB() {
	log.Info("Starting the performance eval for TimescaleDB")
	ticker := time.Tick(60 * time.Second)

	for {
		metricQueries := []datasource.MetricQuery{
			{AwsName: "CPUUtilization", ReportingName: "cpu.utilization.avg", Stat: "Average"},
			{AwsName: "NetworkIn", ReportingName: "network.in.bytes", Stat: "Sum"},
			{AwsName: "NetworkOut", ReportingName: "network.out.bytes", Stat: "Sum"},

			{AwsName: "DiskReadBytes", ReportingName: "disk.read.bytes", Stat: "Sum"},
			{AwsName: "DiskWriteBytes", ReportingName: "disk.write.bytes", Stat: "Sum"},

			{AwsName: "EBSReadBytes", ReportingName: "ebs.read.bytes", Stat: "Sum"},
			{AwsName: "EBSWriteBytes", ReportingName: "ebs.write.bytes", Stat: "Sum"},
		}
		dimensionQueries := []datasource.DimensionQuery{
			{AwsName: "AutoScalingGroupName", ReportingName: "infra.tsdb-asg-", DimensionValues: datasource.AWSProxyInstance.AsgNames},
			{AwsName: "InstanceId", ReportingName: "infra.tsdb-node-", DimensionValues: datasource.AWSProxyInstance.InstanceIDs},
		}
		for _, metricQuery := range metricQueries {
			for dimensionIndex, dimensionQuery := range dimensionQueries {
				for _, dimensionValue := range dimensionQuery.DimensionValues {
					dimensionReportingNameFormatted := fmt.Sprintf("%s%d", dimensionQuery.ReportingName, dimensionIndex+1)
					metricFullname := fmt.Sprintf("%s.%s.%s", config.SandboxID, dimensionReportingNameFormatted, metricQuery.ReportingName)
					if dimensionValue != "" {
						go datasource.AWSProxyInstance.GrabAWSmetric(metricFullname, metricQuery.AwsName, "AWS/EC2", dimensionQuery.AwsName, dimensionValue, metricQuery.Stat)
					}
				}
			}
		}
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.TimescaleDBQuery("metrics-count", timescaleQueryMetricCount, int64(300))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.TimescaleDBQuery("1-ts-24-hour-range", timescaleQuery1Timeserie, int64(60*60*24))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.TimescaleDBQuery("1-ts-1-week-range", timescaleQuery1Timeserie, int64(60*60*24*7))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.TimescaleDBQuery("100-ts-1-week-range", timescaleQuery100Timeseries, int64(60*60*24*7))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.TimescaleDBQuery("400-ts-1-week-range", timescaleQuery400Timeseries, int64(60*60*24*7))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.TimescaleDBQuery("100-ts-6-hour-range", timescaleQuery100Timeseries, int64(60*60*6))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.TimescaleDBQuery("400-ts-6-hour-range", timescaleQuery400Timeseries, int64(60*60*6))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.TimescaleDBQuery("100-ts-p99-1-week-range", timescaleQuery100TimeseriesP99, int64(60*60*24*7))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.TimescaleDBQuery("400-ts-p99-1-week-range", timescaleQuery400TimeseriesP99, int64(60*60*24*7))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.TimescaleDBQuery("100-ts-mean-1-week-range", timescaleQuery100TimeseriesMean, int64(60*60*24*7))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.TimescaleDBQuery("400-ts-mean-1-week-range", timescaleQuery400TimeseriesMean, int64(60*60*24*7))

		select {
		// Need a control channel here
		case <-ticker:
			log.Trace("Ticker ticked")
			continue
		}
	}
}
