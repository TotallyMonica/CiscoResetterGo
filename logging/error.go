package crglogging

type Errorf func(string, ...interface{})
type Errorln func(string)
type Error func(string)

func (l Crglogging) Errorf(format string, args ...interface{}) {
	Logger.Errorf(format, args...)
	l.ErrorCount += 1
}

func (l Crglogging) Errorln(format string) {
	Logger.Errorf("%s\n", format)
	l.ErrorCount += 1
}

func (l Crglogging) Error(format string) {
	Logger.Error(format)
	l.ErrorCount += 1
}
