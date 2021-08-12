package nebulagraph

import (
	"go.k6.io/k6/js/modules"
)

const version = "v0.0.6"

func init() {
	modules.Register("k6/x/nebulagraph", New())
}
