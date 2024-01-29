package common

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnv(t *testing.T) {
	os.Setenv("NEBULA_STMT_PREFIX", "nebula")
	getEnv()
	assert.Equal(t, "nebula", nebulaEnv.NebulaStmtPrefix)
}

func TestProcessStmt(t *testing.T) {
	os.Setenv("NEBULA_STMT_PREFIX", "nebula")
	getEnv()
	assert.Equal(t, "nebula stmt", ProcessStmt("stmt"))
}
