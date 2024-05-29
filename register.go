package k6plugin

import (
	"github.com/vesoft-inc/k6-plugin/pkg/aggcsv"
	"github.com/vesoft-inc/k6-plugin/pkg/nebulagraph"
	"github.com/vesoft-inc/k6-plugin/pkg/nebulameta"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/output"
)

func init() {
	modules.Register("k6/x/nebulagraph", nebulagraph.NewModule())
	modules.Register("k6/x/nebulameta", nebulameta.New())
	output.RegisterExtension("aggcsv", func(p output.Params) (output.Output, error) {
		return aggcsv.New(p)
	})
}
