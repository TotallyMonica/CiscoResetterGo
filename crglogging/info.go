package crglogging

type Infof func(string, ...interface{})
type Infoln func(string)
type Info func(string)

func (l *Crglogging) Infof(format string, args ...interface{}) {
	l.logger.Infof(format, args...)
	l.InfoCount += 1
}

func (l *Crglogging) Infoln(format ...interface{}) {
	l.logger.Infof("%s\n", format)
	l.InfoCount += 1
}

func (l *Crglogging) Info(format ...interface{}) {
	l.logger.Info(format)
	l.InfoCount += 1
}
