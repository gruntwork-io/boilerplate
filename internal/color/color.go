// Package color provides lightweight ANSI terminal color helpers.
package color

const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiHiGreen = "\033[92m"
)

func Green(s string) string     { return ansiGreen + s + ansiReset }
func Yellow(s string) string    { return ansiYellow + s + ansiReset }
func Red(s string) string       { return ansiRed + s + ansiReset }
func BoldGreen(s string) string { return ansiBold + ansiHiGreen + s + ansiReset }
