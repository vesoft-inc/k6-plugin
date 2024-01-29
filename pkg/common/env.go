package common

import "github.com/kelseyhightower/envconfig"

type Environment struct {
	NebulaStmtPrefix string `envconfig:"STMT_PREFIX" `
}

var nebulaEnv *Environment

func getEnv() {
	var env Environment
	err := envconfig.Process("nebula", &env)
	if err != nil {
		panic(err)
	}
	nebulaEnv = &env
}

func init() {
	getEnv()
}

func ProcessStmt(stmt string) string {
	if nebulaEnv.NebulaStmtPrefix != "" {
		stmt = nebulaEnv.NebulaStmtPrefix + " " + stmt
	}
	return stmt
}
