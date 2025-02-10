package crglogging

type Debugf func(string, ...interface{})
type Debugln func(string)
type Debug func(string)

func (l Crglogging) Debugf(format string, args ...interface{}) {
	l.logger.Debugf(format, args...)
	l.DebugCount += 1
}

func (l Crglogging) Debugln(format ...interface{}) {
	l.logger.Debugf("%s\n", format)
	l.DebugCount += 1
}

func (l Crglogging) Debug(format ...interface{}) {
	l.logger.Debug(format)
	l.DebugCount += 1
}
