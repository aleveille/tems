# TEMS

## What is TSDB perf eval

This tool is intended to query TSDB(s) in a neutral, predictable and repeatable
way in order to compare different TSDBs or different configurations of the same
TSDB. It is intended to be used hand in hand with
[Lagrande](https://github.com/aleveille/lagrande).

## Getting started

This tool assume you have access to:

* A Grafana instance with a TSDB datasource configured
* [Read-only credentials](#aws-access) to an AWS account
* A [Circonus SaaS account](https://www.circonus.com/) (free tier OK)

`./tems -awsProfile myprofile -sandboxID someid -circonusAPIToken 000a000e-a00a-0000-000a-0a00a0a0a000 -grafanaURL https://grafana.sandbox.mydomain.com -grafanaPassword something`

## How it works

The tool will send queries through Grafana as a proxy as if you had a dashboard
or users looking at data from your TSDB with Grafana. It will at the same time
pull some infrastructure-level metrics such as CPU usage, network bandwidth,
etc.

This allows to see how long queries are taking (and if they are reporting the 
expected value) as more and more data gets ingested in the TSDB.

## AWS access

The following policy is enough for the needs of the program. The action
`autoscaling:Describe*` is used to find the complete name of the ASG used in
the TSDB sandbox and their instances' IDs and then grab metrics from them.

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "cloudwatch:GetMetricData",
                "autoscaling:DescribeAutoScalingGroups"
            ],
            "Resource": "*"
        }
    ]
}
```

## Status

IRONdb and InfluxDB (WIP) are the only supported TSDB so far. I have plenty
more to implement on my to-do list, but if you have a special requirement or
would like me to fast track one of them, let me know.

## Name

TEMS gets its name from [Turbine Efficiency Monitoring Systems](https://www.wateronline.com/doc/hydroelectric-power-station-monitoring-system-0001).
Since its peer program ([Lagrande](https://github.com/aleveille/lagrande)) name
is hydroelectricity themed, TEMS follows suit with something related to
monitoring efficiency.


## Contributing

If you have a feature request or a question, feel free to open an issue. Otherwise, I accept contributions via GitHub pull requests.

## License

MIT License

Copyright (c) 2019 Alexandre Léveillé

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.