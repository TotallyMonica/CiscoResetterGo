package crglogging

type Errorf func(string, ...interface{})
type Errorln func(string)
type Error func(string)

func (l Crglogging) Errorf(format string, args ...interface{}) {
	l.logger.Errorf(format, args...)
	l.ErrorCount += 1
}

func (l Crglogging) Errorln(format ...interface{}) {
	l.logger.Errorf("%s\n", format)
	l.ErrorCount += 1
}

func (l Crglogging) Error(format ...interface{}) {
	l.logger.Error(format)
	l.ErrorCount += 1
}
