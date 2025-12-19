package nebulagraph5

import (
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/js/modules"
)

var _ modules.Module = &K6Module{}

// refer: https://k6.io/docs/extensions/get-started/create/javascript-extensions/#use-the-advanced-module-api
// K6Module is a module for k6, using the advanced module API
type K6Module struct {
	pool *GraphPool
}

type K6NebulaInstance struct {
	vu   modules.VU
	pool *GraphPool
}

type loggerWrapper struct {
	log logrus.FieldLogger
}

func (l *loggerWrapper) Infof(msg string, args ...any) {
	l.log.Infof(msg, args...)
}

func (l *loggerWrapper) Warnf(msg string, args ...any) {
	l.log.Warnf(msg, args...)
}

func (l *loggerWrapper) Debugf(msg string, args ...any) {
	l.log.Debugf(msg, args...)
}

func (l *loggerWrapper) Errorf(msg string, args ...any) {
	l.log.Errorf(msg, args...)
}

func NewModule() *K6Module {
	return &K6Module{
		pool: NewNebulaGraph(),
	}
}

func (m *K6Module) NewModuleInstance(vu modules.VU) modules.Instance {
	return &K6NebulaInstance{
		vu:   vu,
		pool: m.pool,
	}
}

func (i *K6NebulaInstance) Exports() modules.Exports {
	logger := i.vu.InitEnv().Logger
	i.pool.logger = &loggerWrapper{log: logger}
	return modules.Exports{
		Default: i.pool,
	}
}
