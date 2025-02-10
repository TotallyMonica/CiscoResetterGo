package crglogging

type Warnf func(string, ...interface{})
type Warnln func(string)
type Warn func(string)

func (l Crglogging) Warnf(format string, args ...interface{}) {
	Logger.Warningf(format, args...)
	l.WarnCount += 1
}

func (l Crglogging) Warnln(format string) {
	Logger.Warningf("%s\n", format)
	l.WarnCount += 1
}

func (l Crglogging) Warn(format string) {
	Logger.Warning(format)
	l.WarnCount += 1
}
