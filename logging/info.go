package crglogging

type Infof func(string, ...interface{})
type Infoln func(string)
type Info func(string)

func (l Crglogging) Infof(format string, args ...interface{}) {
	Logger.Infof(format, args...)
	l.InfoCount += 1
}

func (l Crglogging) Infoln(format string) {
	Logger.Infof("%s\n", format)
	l.InfoCount += 1
}

func (l Crglogging) Info(format string) {
	Logger.Info(format)
	l.InfoCount += 1
}
