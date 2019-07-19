package dataout

import (
	"fmt"

	log "github.com/aleveille/tems/logger"
)

var (
	// ResultChan is where each data sources will publish the data they gather
	ResultChan chan Result
)

// Result is an abstration of a datapoint result to be pushed to Circonus SaaS platform
// It's closely tied to the IRONdb TSDB data model: the timestamp of the measurement, the metric name, a value and the datatype of the value
type Result struct {
	Timestamp int64
	Name      string
	Value     string
	// datatype value:
	// https://login.circonus.com/resources/docs/user/Data/CheckTypes/Resmon.html
	// Guess: i = int, I = int64, l = uint, L = uint64, n = numeric (float?), s = string
	Datatype string
}

// InitResultChan initialize the result channel
func InitResultChan() error {
	log.Debug("InitResultChan() start")
	defer log.Debug("InitResultChan() end")
	ResultChan = make(chan Result, 10000)

	go handleResult()

	return nil
}

// ToString formats the result in a human-readable and machine-parsable string
func (r *Result) ToString() string {
	return fmt.Sprintf("[%d] %s=%s", r.Timestamp, r.Name, r.Value)
}

// HandleResult will receive a Result struct and handle it (log it + pass it to CirconusProxyInstance so it can be uploaded)
func handleResult() {
	for {
		select {
		case result := <-ResultChan:
			log.PrintToResultLog(result.ToString())
			dt := result.Datatype
			if dt == "" {
				dt = "n"
			}
			val := result.Value
			if val == "nan" || val == "" || val == "null" {
				log.Warnf("Skipping result %s for %s\n", val, result.Name)
				break
			} else {
				log.Tracef("Got result: %s", result.ToString())
			}
			CirconusProxyInstance.PushDatapoint(result.Timestamp, result.Name, val, dt)
		}
	}
}
