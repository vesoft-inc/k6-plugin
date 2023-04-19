// inspired by https://github.com/bygui86/go-csv-view/tree/main/examples/two-y-axis

package main

import (
	"embed"
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
)

//go:embed js
var jsFile embed.FS

var defaultDraw = &draw{}

type draw struct {
	filePath   string
	output     string
	percentile string
	data       *drawData
}

type drawData struct {
	timestamp    []string
	vu           []int
	requestCount []int
	errorCount   []int
	responseTime []float32
	latency      []float32
}

// #timestamp,vus,requestCount,errorCount,latencyAvg,latencyP90,latencyP95,latencyP99,responseTimeAvg,responseTimeP90,responseTimeP95,responseTimeP99,rowSizePerReq
func (d *draw) init() error {
	bs, err := os.OpenFile(d.filePath, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	d.data = &drawData{
		timestamp:    make([]string, 0, 100),
		vu:           make([]int, 0, 100),
		requestCount: make([]int, 0, 100),
		errorCount:   make([]int, 0, 100),
		responseTime: make([]float32, 0, 100),
		latency:      make([]float32, 0, 100),
	}
	var latencyPos, responsePos int
	switch d.percentile {
	case "avg":
		latencyPos = 4
		responsePos = 8
	case "p90":
		latencyPos = 5
		responsePos = 9
	case "p95":
		latencyPos = 6
		responsePos = 10
	case "p99":
		latencyPos = 7
		responsePos = 11
	default:
		return fmt.Errorf("invalid percentile: %s", d.percentile)
	}

	r := csv.NewReader(bs)
	r.Comma = []rune(",")[0]
	// skip header
	_, _ = r.Read()
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		vu, _ := strconv.Atoi(record[1])
		rc, _ := strconv.Atoi(record[2])
		ec, _ := strconv.Atoi(record[3])
		latency, _ := strconv.ParseFloat(record[latencyPos], 32)
		response, _ := strconv.ParseFloat(record[responsePos], 32)

		d.data.timestamp = append(d.data.timestamp, record[0])
		d.data.vu = append(d.data.vu, vu)
		d.data.requestCount = append(d.data.requestCount, rc)
		d.data.errorCount = append(d.data.errorCount, ec)
		d.data.latency = append(d.data.latency, float32(latency))
		d.data.responseTime = append(d.data.responseTime, float32(response))
	}

	return nil
}

func (d *draw) render() error {
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title: "K6 Result Chart",
		}),
		charts.WithLegendOpts(opts.Legend{
			Show:         true,
			SelectedMode: "multiple",
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    true,
			Trigger: "axis",
			AxisPointer: &opts.AxisPointer{
				Type: "cross",
				Snap: true,
			},
		}),
		// AXIS
		charts.WithXAxisOpts(opts.XAxis{
			SplitNumber: 20,
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name:  "Count",
			Type:  "value",
			Show:  true,
			Scale: true,
		}),
	)

	line.ExtendYAxis(opts.YAxis{
		Name:  "duration(ms)",
		Type:  "value",
		Show:  true,
		Scale: true,
	})

	line.SetXAxis(d.data.timestamp)

	line.AddSeries("Request Count", d.generate(d.data.requestCount, 0), charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	line.AddSeries("Error Count", d.generate(d.data.errorCount, 0), charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	line.AddSeries("Latency(ms)", d.generate(d.data.latency, 1), charts.WithLineChartOpts(opts.LineChart{Smooth: true, YAxisIndex: 1}))
	line.AddSeries("ResponseTime(ms)", d.generate(d.data.responseTime, 1), charts.WithLineChartOpts(opts.LineChart{Smooth: true, YAxisIndex: 1}))
	f, err := os.Create(d.output)
	if err != nil {
		return err
	}

	page := components.NewPage()
	page.JSAssets.Values = []string{}
	page.CustomizedJSAssets.Values = []string{"./echarts.min.js"}
	page.AddCharts(line)
	page.Render(f)
	// save js file
	js, _ := jsFile.ReadFile("js/echarts.min.js")
	_ = ioutil.WriteFile("echarts.min.js", js, 0644)
	return nil
}

func (d *draw) generate(data interface{}, yAxisIndex int) []opts.LineData {
	var res []opts.LineData
	switch data.(type) {
	case []int:
		for _, v := range data.([]int) {
			res = append(res, opts.LineData{Value: v, YAxisIndex: yAxisIndex})
		}
	case []float32:
		for _, v := range data.([]float32) {
			res = append(res, opts.LineData{Value: v, YAxisIndex: yAxisIndex})
		}
	}
	return res
}
