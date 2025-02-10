package crglogging

type Fatalf func(string, ...interface{})
type Fatalln func(string)
type Fatal func(string)

func (l Crglogging) Fatalf(format string, args ...interface{}) {
	Logger.Fatalf(format, args...)
	l.FatalCount += 1
}

func (l Crglogging) Fatalln(format string) {
	Logger.Fatalf("%s\n", format)
	l.FatalCount += 1
}

func (l Crglogging) Fatal(format string) {
	Logger.Fatal(format)
	l.FatalCount += 1
}
