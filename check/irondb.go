package check

import (
	"fmt"
	"time"

	"github.com/aleveille/tems/config"
	"github.com/aleveille/tems/datasource"

	log "github.com/aleveille/tems/logger"
)

var (
	caqlQueryMetricCount   = "find(%22%2Flagrande.randomint-1%5C.lg%5B0-4%5D%5C.%2F%22)%7Ccount()"
	caqlQuery1Timeserie    = "find(%22lagrande.randomint-1.lg1.1%22)"
	caqlQuery100Timeseries = "find(%22lagrande.randomint-1.lg1.1%3F%3F%22)"
	caqlQuery400Timeseries = "find(%22%2Flagrande.randomint-1.lg%5B0-4%5D.1%5B0-9%5D%7B2%7D%2F%22)"
	//find("lagrande.latency.lg1.*")|Stats:percentile(99)
	caqlQuery100TimeseriesP99  = "find(%22lagrande.randomint-1.lg1.1%3F%3F%22)%7Cwindow%3Apercentile(1M%2C%2099)"
	caqlQuery400TimeseriesP99  = "find(%22%2Flagrande.randomint-1.lg%5B0-4%5D.1%5B0-9%5D%7B2%7D%2F%22)%7Cwindow%3Apercentile(1M%2C%2099)"
	caqlQuery100TimeseriesMean = "find(%22lagrande.randomint-1.lg1.1%3F%3F%22)%7Cwindow%3Amean(1M)"
	caqlQuery400TimeseriesMean = "find(%22%2Flagrande.randomint-1.lg%5B0-4%5D.1%5B0-9%5D%7B2%7D%2F%22)%7Cwindow%3Amean(1M)"

	caqlQueryMetricCountTags   = "find(%22randomint-1%22%2C%22and(namespace%3Alagrande%2Cnode%3A%2Flg%5B0-4%5D%2F)%22)%7Ccount()"
	caqlQuery1TimeserieTags    = "find(%22randomint-1%22%2C%22and(namespace%3Alagrande%2Cnode%3Alg1%2Cworker%3A1)%22)"
	caqlQuery100TimeseriesTags = "find(%22randomint-1%22%2C%22and(namespace%3Alagrande%2Cnode%3Alg1%2Cworker%3A1%3F%3F)%22)"
	caqlQuery400TimeseriesTags = "find(%22randomint-1%22%2C%22and(namespace%3Alagrande%2Cnode%3A%2Flg%5B0-4%5D%2F%2Cworker%3A1%3F%3F)%22)"
	//find("lagrande.latency.lg1.*")|Stats:percentile(99)
	caqlQuery100TimeseriesP99Tags  = "find(%22randomint-1%22%2C%22and(namespace%3Alagrande%2Cnode%3Alg1%2Cworker%3A1%3F%3F)%22)%7Cwindow%3Apercentile(1M%2C%2099)"
	caqlQuery400TimeseriesP99Tags  = "find(%22randomint-1%22%2C%22and(namespace%3Alagrande%2Cnode%3A%2Flg%5B0-4%5D%2F%2Cworker%3A1%3F%3F)%22)%7Cwindow%3Apercentile(1M%2C%2099)"
	caqlQuery100TimeseriesMeanTags = "find(%22randomint-1%22%2C%22and(namespace%3Alagrande%2Cnode%3Alg1%2Cworker%3A1%3F%3F)%22)%7Cwindow%3Amean(1M)"
	caqlQuery400TimeseriesMeanTags = "find(%22randomint-1%22%2C%22and(namespace%3Alagrande%2Cnode%3A%2Flg%5B0-4%5D%2F%2Cworker%3A1%3F%3F)%22)%7Cwindow%3Amean(1M)"
)

// EvaluateIRONdb will launch the AWS and CAQL queries to assess IRONdb performances
func EvaluateIRONdb() {
	log.Info("Starting the performance eval for IRONdb")
	ticker := time.Tick(60 * time.Second)

	if config.CAQLUseTags {
		log.Info("Using CAQL queries with tag support")
		caqlQueryMetricCount = caqlQueryMetricCountTags
		caqlQuery1Timeserie = caqlQuery1TimeserieTags
		caqlQuery100Timeseries = caqlQuery100TimeseriesTags
		caqlQuery400Timeseries = caqlQuery400TimeseriesTags
		caqlQuery100TimeseriesP99 = caqlQuery100TimeseriesP99Tags
		caqlQuery400TimeseriesP99 = caqlQuery400TimeseriesP99Tags
		caqlQuery100TimeseriesMean = caqlQuery100TimeseriesMeanTags
		caqlQuery400TimeseriesMean = caqlQuery400TimeseriesMeanTags
	}

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

		go datasource.GrafanaProxyInstance.SimpleCaqlQuery("metrics-count", caqlQueryMetricCount, int64(300))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleCaqlQuery("1-ts-24-hour-range", caqlQuery1Timeserie, int64(60*60*24))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleCaqlQuery("1-ts-1-week-range", caqlQuery1Timeserie, int64(60*60*24*7))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleCaqlQuery("100-ts-1-week-range", caqlQuery100Timeseries, int64(60*60*24*7))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleCaqlQuery("400-ts-1-week-range", caqlQuery400Timeseries, int64(60*60*24*7))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleCaqlQuery("100-ts-6-hour-range", caqlQuery100Timeseries, int64(60*60*6))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleCaqlQuery("400-ts-6-hour-range", caqlQuery400Timeseries, int64(60*60*6))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleCaqlQuery("100-ts-p99-1-week-range", caqlQuery100TimeseriesP99, int64(60*60*24*7))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleCaqlQuery("400-ts-p99-1-week-range", caqlQuery400TimeseriesP99, int64(60*60*24*7))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleCaqlQuery("100-ts-mean-1-week-range", caqlQuery100TimeseriesMean, int64(60*60*24*7))
		time.Sleep(2 * time.Second)
		go datasource.GrafanaProxyInstance.SimpleCaqlQuery("400-ts-mean-1-week-range", caqlQuery400TimeseriesMean, int64(60*60*24*7))

		select {
		// Need a control channel here
		case <-ticker:
			log.Trace("Ticker ticked")
			continue
		}
	}
}
