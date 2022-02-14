package nebulagraph

import (
	"go.k6.io/k6/js/modules"
)

func init() {
	modules.Register("k6/x/nebulagraph", New())
}
