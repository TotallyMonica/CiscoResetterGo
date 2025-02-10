package crglogging

type Warnf func(string, ...interface{})
type Warnln func(string)
type Warn func(string)

func (l Crglogging) Warnf(format string, args ...interface{}) {
	l.logger.Warningf(format, args...)
	l.WarnCount += 1
}

func (l Crglogging) Warnln(format string) {
	l.logger.Warningf("%s\n", format)
	l.WarnCount += 1
}

func (l Crglogging) Warn(format ...interface{}) {
	l.logger.Warning(format)
	l.WarnCount += 1
}
