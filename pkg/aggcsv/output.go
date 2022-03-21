package aggcsv

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"go.k6.io/k6/output"
)

type Config struct {
	outFilePath         string
	aggregationInterval int32 // in seconds, default 5s
}

type Output struct {
	output.SampleBuffer
	config          Config
	outputFile      *os.File
	periodicFlusher *output.PeriodicFlusher
}

func New(params output.Params) (*Output, error) {
	var interval int32 = 5
	aggInterval := os.Getenv("AGGREGATION_INTERVAL")
	if aggInterval != "" {
		_interval, err := strconv.ParseInt(aggInterval, 10, 64)
		if err != nil {
			return nil, err
		}
		interval = int32(_interval)
	}

	config := Config{
		outFilePath:         params.ConfigArgument,
		aggregationInterval: interval,
	}

	return &Output{
		config: config,
	}, nil
}

func (o *Output) Description() string {
	return "aggregation csv output"
}

func (o *Output) Start() error {
	var err error
	o.outputFile, err = os.Create(o.config.outFilePath)
	if err != nil {
		return err
	}
	o.outputFile.Write([]byte("#timestamp,vus,requestCount,errorCount,latencyAvg,latencyP90,latencyP95,latencyP99,responseTimeAvg,responseTimeP90,responseTimeP95,responseTimeP99,rowSizePerReq\n"))

	pf, err := output.NewPeriodicFlusher(time.Duration(o.config.aggregationInterval)*time.Second, o.aggregateAndFlush)
	if err != nil {
		o.outputFile.Close()
		return err
	}
	o.periodicFlusher = pf

	return nil
}

func (o *Output) Stop() error {
	o.periodicFlusher.Stop()
	o.outputFile.Close()
	return nil
}

func (o *Output) aggregateAndFlush() {
	sampleContainers := o.GetBufferedSamples()
	if len(sampleContainers) == 0 {
		return
	}

	var latencies []float64
	var rts []float64
	var vus int64
	var requestCount int64
	var errorCount int64
	var rowSize int64

	for _, container := range sampleContainers {
		for _, sample := range container.GetSamples() {
			value := sample.Value
			switch sample.Metric.Name {
			case "vus":
				intValue := int64(value)
				if intValue > vus {
					vus = intValue
				}
			case "checks":
				requestCount += 1
				if int64(value) == 0 {
					errorCount += 1
				}
			case "latency":
				latencies = append(latencies, value/1000.0)
			case "responseTime":
				rts = append(rts, value/1000.0)
			case "rowSize":
				rowSize += int64(value)
			}
		}
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})
	sort.Slice(rts, func(i, j int) bool {
		return rts[i] < rts[j]
	})

	var latencyAvg, latencyP90, latencyP95, latencyP99 float64
	if len(latencies) > 0 {
		latencyLen := float64(len(latencies))
		latencyAvg = average(latencies)
		latencyP90 = latencies[int(latencyLen*0.90)]
		latencyP95 = latencies[int(latencyLen*0.95)]
		latencyP99 = latencies[int(latencyLen*0.99)]
	}

	var rtAvg, rtP90, rtP95, rtP99 float64
	if len(rts) > 0 {
		rtLen := float64(len(rts))
		rtAvg = average(rts)
		rtP90 = rts[int(rtLen*0.90)]
		rtP95 = rts[int(rtLen*0.95)]
		rtP99 = rts[int(rtLen*0.99)]
	}

	var rowSizePerReq int64 = 0
	if requestCount > 0 {
		rowSizePerReq = rowSize / requestCount
	}

	line := fmt.Sprintf("%d,%d,%d,%d,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%d\n",
		time.Now().UnixNano()/int64(time.Millisecond), vus, requestCount, errorCount,
		latencyAvg, latencyP90, latencyP95, latencyP99,
		rtAvg, rtP90, rtP95, rtP99,
		rowSizePerReq)
	o.outputFile.Write([]byte(line))
}

func average(xs []float64) float64 {
	total := 0.0
	for _, v := range xs {
		total += v
	}
	return total / float64(len(xs))
}
