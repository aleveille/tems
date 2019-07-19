package check

import (
	"fmt"
	"time"

	"github.com/aleveille/tems/config"
	"github.com/aleveille/tems/datasource"

	log "github.com/aleveille/tems/logger"
)

var (
	influxdbQueryMetricCount = `from(bucket: "mydb/autogen") 
									|> range(start: -8m, stop: -3m)
									|> filter(fn: (r) => r.run == "4" and r.process == "lagrande" and (r._field == "value"))
									|> map(fn: (r) => ({ _time: r._time, fqn: r.process + "." +  r.node + "." + r._measurement + "." + r.worker }))
									|> keep(columns: ["_time", "fqn"])
									|> window(every: 1m)
									|> unique(column: "fqn")
									|> aggregateWindow(every: 1m, fn: count, columns: ["fqn"])`
	influxdbQuery1Timeserie    = `SELECT "value" FROM "randomint-1" WHERE ("worker" = '1' AND "node" = 'lg1') AND time >= now() - %s`
	influxdbQuery100Timeseries = `SELECT "value" FROM "randomint-1" WHERE ("worker" =~ /1[0-9]{2}/ AND "node" = 'lg1') AND time >= now() - %s`
	influxdbQuery400Timeseries = `SELECT "value" FROM "randomint-1" WHERE ("worker" =~ /1[0-9]{2}/ AND "node" =~ /lg[0-4]/) AND time >= now() - %s`
	//find("lagrande.latency.lg1.*")|Stats:percentile(99)
	influxdbQuery100TimeseriesP99  = `SELECT percentile("value", 99) FROM "randomint-1" WHERE ("worker" =~ /1[0-9]{2}/ AND "node" = 'lg1') AND time >= now() - %s GROUP BY time(5s)`
	influxdbQuery400TimeseriesP99  = `SELECT percentile("value", 99) FROM "randomint-1" WHERE ("worker" =~ /1[0-9]{2}/ AND "node" =~ /lg[0-4]/) AND time >= now() - %s GROUP BY time(5s)`
	influxdbQuery100TimeseriesMean = `SELECT mean("value") FROM "randomint-1" WHERE ("worker" =~ /1[0-9]{2}/ AND "node" = 'lg1') AND time >= now() - %s GROUP BY time(5s)`
	influxdbQuery400TimeseriesMean = `SELECT mean("value") FROM "randomint-1" WHERE ("worker" =~ /1[0-9]{2}/ AND "node" =~ /lg[0-4]/) AND time >= now() - %s GROUP BY time(5s)`
)

// EvaluateInfluxDB will launch the AWS and InfluxDB queries to assess InfluxDB performances
func EvaluateInfluxDB() {
	log.Info("Starting the performance eval for InfluxDB")
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

		go datasource.GrafanaProxyInstance.SimpleInfluxDBQuery("metrics-count", influxdbQueryMetricCount, "5m")
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleInfluxDBQuery("1-ts-24-hour-range", influxdbQuery1Timeserie, "24h")
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleInfluxDBQuery("1-ts-1-week-range", influxdbQuery1Timeserie, "7d")
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleInfluxDBQuery("100-ts-1-week-range", influxdbQuery100Timeseries, "7d")
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleInfluxDBQuery("400-ts-1-week-range", influxdbQuery400Timeseries, "7d")
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleInfluxDBQuery("100-ts-6-hour-range", influxdbQuery100Timeseries, "6h")
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleInfluxDBQuery("400-ts-6-hour-range", influxdbQuery400Timeseries, "6h")
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleInfluxDBQuery("100-ts-p99-1-week-range", influxdbQuery100TimeseriesP99, "7d")
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleInfluxDBQuery("400-ts-p99-1-week-range", influxdbQuery400TimeseriesP99, "7d")
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleInfluxDBQuery("100-ts-mean-1-week-range", influxdbQuery100TimeseriesMean, "7d")
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleInfluxDBQuery("400-ts-mean-1-week-range", influxdbQuery400TimeseriesMean, "7d")

		select {
		// Need a control channel here
		case <-ticker:
			log.Trace("Ticker ticked")
			continue
		}
	}
}
