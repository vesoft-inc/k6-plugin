package k6plugin

import (
	"github.com/vesoft-inc/k6-plugin/pkg/nebulagraph"
	"go.k6.io/k6/js/modules"
)

func init() {
	modules.Register("k6/x/nebulagraph", nebulagraph.NewNebulaGraph())
}
