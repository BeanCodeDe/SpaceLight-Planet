package server

type (
	Logger interface {
		Debugf(format string, args ...interface{})
		Infof(format string, args ...interface{})
		Warnf(format string, args ...interface{})
		Errorf(format string, args ...interface{})
	}
	nilLogger struct {
	}
)

func (log nilLogger) Debugf(format string, args ...interface{}) {}
func (log nilLogger) Infof(format string, args ...interface{})  {}
func (log nilLogger) Warnf(format string, args ...interface{})  {}
func (log nilLogger) Errorf(format string, args ...interface{}) {}
