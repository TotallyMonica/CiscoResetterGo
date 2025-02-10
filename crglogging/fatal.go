package crglogging

type Fatalf func(string, ...interface{})
type Fatalln func(string)
type Fatal func(string)

func (l Crglogging) Fatalf(format string, args ...interface{}) {
	l.logger.Fatalf(format, args...)
	l.FatalCount += 1
}

func (l Crglogging) Fatalln(format ...interface{}) {
	l.logger.Fatalf("%s\n", format)
	l.FatalCount += 1
}

func (l Crglogging) Fatal(format ...interface{}) {
	l.logger.Fatal(format)
	l.FatalCount += 1
}
