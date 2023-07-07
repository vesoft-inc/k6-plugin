module github.com/vesoft-inc/k6-plugin

go 1.16

require (
	github.com/go-echarts/go-echarts/v2 v2.2.4
	github.com/spf13/cobra v1.4.0
	github.com/vesoft-inc/nebula-go/v3 v3.5.1-0.20230613062129-8a5ad7e936f7
	go.k6.io/k6 v0.43.0
)

replace github.com/facebook/fbthrift => github.com/vesoft-inc/fbthrift v0.0.0-20230214024353-fa2f34755b28
