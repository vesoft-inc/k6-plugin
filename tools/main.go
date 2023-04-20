package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var drawCmd = &cobra.Command{
	Use:   "draw",
	Short: "Draw chart for k6 results",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		if defaultDraw.filePath == "" {
			return fmt.Errorf("file path is required")
		}
		if defaultDraw.output == "" {
			return fmt.Errorf("output path is required")
		}
		if err := defaultDraw.init(); err != nil {
			return err
		}
		if err := defaultDraw.render(); err != nil {
			return err
		}
		return nil
	},
}

func main() {
	drawCmd.Execute()
}

func init() {
	drawCmd.Flags().StringVarP(&defaultDraw.filePath, "file", "f", "", "k6 result file path")
	drawCmd.Flags().StringVarP(&defaultDraw.output, "output", "o", "", "output file path")
	drawCmd.Flags().StringVarP(&defaultDraw.percentile, "percentile", "p", "p95",
		"percentile for latency and response time, e.g. avg, p90, p95, p99")
}
