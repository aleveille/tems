package datasource

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/cloudwatch"

	"github.com/aleveille/tems/config"
	"github.com/aleveille/tems/dataout"
	appError "github.com/aleveille/tems/error"
	log "github.com/aleveille/tems/logger"
)

var (
	// AWSProxyInstance is the globally accessible AWSProxy struct
	AWSProxyInstance AWSProxy
)

// AWSProxy is our wrapper to provide higher-level functionnality around the AWS Go SDK
// It maintains its session and the required service instances
type AWSProxy struct {
	awsSession *session.Session

	cloudwatchService  *cloudwatch.CloudWatch
	autoscalingService *autoscaling.AutoScaling

	AsgNames    []string
	InstanceIDs []string
}

type DimensionQuery struct {
	AwsName         string
	ReportingName   string
	DimensionValues []string
}

type MetricQuery struct {
	AwsName       string
	ReportingName string
	Stat          string
}

// InitAWSProxy initialize the AWSProxy struct in order to interact with AWS API
func InitAWSProxy() (*AWSProxy, error) {
	log.Debug("InitAWSProxy() start")
	defer log.Debug("InitAWSProxy() end")
	proxy := AWSProxy{}
	var err error

	proxy.AsgNames = make([]string, config.AWSExpectedASGs)
	proxy.InstanceIDs = make([]string, config.AWSExpectedASGs*config.AWSExpectedInstanceCountPerASG)

	err = proxy.openSession()
	if err != nil {
		return &proxy, err
	}

	err = proxy.validateSession()
	if err != nil {
		return &proxy, err
	}

	log.Debugf("AWS: Successfully logged in.")

	err = proxy.createServices()
	if err != nil {
		return &proxy, err
	}

	err = proxy.findSandboxIRONdbAutoscalingGroups()
	if err != nil {
		return &proxy, err
	}

	AWSProxyInstance = proxy
	return &proxy, nil
}

func (a *AWSProxy) openSession() error {
	log.Trace("AWS openSession() start")
	defer log.Trace("AWS openSession() end")
	btrue := true
	awsSession, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Profile:           config.AWSProfile,
		Config: aws.Config{
			Region:                        aws.String(config.AWSRegion),
			CredentialsChainVerboseErrors: &btrue,
		},
	})
	a.awsSession = awsSession

	if err != nil {
		return appError.NewInitializationError("couldn't open session with AWS", err)
	}

	return nil
}

func (a *AWSProxy) validateSession() error {
	log.Trace("AWS validateSession() start")
	defer log.Trace("AWS validateSession() end")

	_, credErr := a.awsSession.Config.Credentials.Get()
	if credErr != nil {
		return appError.NewInitializationError("error loading/validating creds", credErr)
	}

	return nil
}

func (a *AWSProxy) createServices() error {
	a.cloudwatchService = cloudwatch.New(a.awsSession)
	a.autoscalingService = autoscaling.New(a.awsSession)

	return nil
}

func (a *AWSProxy) findSandboxIRONdbAutoscalingGroups() error {
	log.Trace("AWS findSandboxIRONdbAutoscalingGroups() start")
	defer log.Trace("AWS findSandboxIRONdbAutoscalingGroups() end")

	sandboxASGPrefix := fmt.Sprintf("%s_%s-nodes", config.SandboxID, config.TSDBSystem)

	foundAsgCount := 0
	var nextToken *string

	// Prevents looping forever with a safe limit
	for page := 0; page < 50; page++ {
		input := &autoscaling.DescribeAutoScalingGroupsInput{
			NextToken: nextToken,
		}

		result, err := a.autoscalingService.DescribeAutoScalingGroups(input)

		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case autoscaling.ErrCodeInvalidNextToken:
					return appError.NewInitializationError(fmt.Sprintf("usage error while looking up IRONdb autoscaling groups: %s", autoscaling.ErrCodeInvalidNextToken), aerr)
				case autoscaling.ErrCodeResourceContentionFault:
					return appError.NewInitializationError(fmt.Sprintf("usage error while looking up IRONdb autoscaling groups: %s", autoscaling.ErrCodeResourceContentionFault), aerr)
				default:
					return appError.NewInitializationError(fmt.Sprintf("error while looking up IRONdb autoscaling groups: %s", aerr.Code), aerr)
				}
			}

			return appError.NewInitializationError("unknown error while looking up IRONdb autoscaling groups", err)
		}

		for _, group := range result.AutoScalingGroups {
			if strings.Contains(*group.AutoScalingGroupName, sandboxASGPrefix) {
				a.AsgNames[foundAsgCount] = *group.AutoScalingGroupName

				for index, inst := range (*group).Instances {
					if index < config.AWSExpectedInstanceCountPerASG {
						instanceNb := foundAsgCount*config.AWSExpectedInstanceCountPerASG + index
						a.InstanceIDs[instanceNb] = *inst.InstanceId
					} else {
						log.Errorf("found too many instances in ASG %s*\n", *group.AutoScalingGroupName)
					}
				}

				foundAsgCount++

				// Break as soon as we've found the expected number of ASG
				if foundAsgCount >= config.AWSExpectedASGs {
					result.NextToken = nil
					break
				}
			}
		}

		if result.NextToken == nil { // No more pages and/or all expected ASG found
			break
		} else {
			nextToken = result.NextToken
		}
	}

	log.Debugf("asgNames: %s\n", a.AsgNames)
	log.Debugf("instanceIDs: %s\n", a.InstanceIDs)

	if len(a.AsgNames) == 0 {
		log.Warn("No ASG found for the sandbox")
	}
	if len(a.InstanceIDs) == 0 {
		log.Warn("No instances found for the sandbox")
	}

	return nil
}

// GrabAWSmetric will query the AWS API to grab a CloudWatch metric
func (a *AWSProxy) GrabAWSmetric(resultMetricName string, awsMetricName string, namespace string, dimensionName string, dimensionValue string, stat string) {
	queryTimestamp := time.Now()

	startTime := queryTimestamp.Add(-61 * time.Second)
	endTime := queryTimestamp

	metricDataQuery := []*cloudwatch.MetricDataQuery{
		&cloudwatch.MetricDataQuery{
			Id: aws.String("queryid"),
			MetricStat: &cloudwatch.MetricStat{
				Metric: &cloudwatch.Metric{
					MetricName: aws.String(awsMetricName),
					Namespace:  aws.String(namespace),
					Dimensions: []*cloudwatch.Dimension{
						&cloudwatch.Dimension{
							Name:  aws.String(dimensionName),
							Value: aws.String(dimensionValue),
						},
					},
				},
				Period: aws.Int64(60),
				Stat:   aws.String(stat),
			},
		},
	}

	out, getErr := a.cloudwatchService.GetMetricData(&cloudwatch.GetMetricDataInput{
		StartTime:         &startTime,
		EndTime:           &endTime,
		MetricDataQueries: metricDataQuery,
	})

	metricTimestamp := int64(0)
	val := "nan"
	if getErr != nil {
		log.Errorf("Error while retrieving metric(s):\n%s\n", getErr)
	} else {
		if out.MetricDataResults != nil && len(out.MetricDataResults) > 0 {
			metricDataResult := *(out.MetricDataResults[0])
			if len(metricDataResult.Values) > 0 {
				val = fmt.Sprintf("%.2f", *metricDataResult.Values[0])
				metricTimestamp = (metricDataResult.Timestamps[0]).Unix()

				r := dataout.Result{Timestamp: metricTimestamp, Name: resultMetricName, Value: val}
				select {
				case dataout.ResultChan <- r:
				default:
					log.Error("Channel full, discarding result")
				}
			} else {
				if awsMetricName != "DiskReadBytes" && awsMetricName != "DiskWriteBytes" && awsMetricName != "EBSReadBytes" && awsMetricName != "EBSkWriteBytes" {
					log.Warnf("No data values retrieved from AWS for %s / %s • %s=%s: %s", namespace, awsMetricName, dimensionName, dimensionValue, stat)
				}
			}
		} else {
			log.Warnf("No data results retrieved from AWS for %s / %s • %s=%s: %s", namespace, awsMetricName, dimensionName, dimensionValue, stat)
		}
	}
}
