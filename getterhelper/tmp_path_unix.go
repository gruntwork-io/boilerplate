//go:build aix || darwin || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris
// +build aix darwin dragonfly freebsd js,wasm linux netbsd openbsd solaris

package getterhelper

import (
	"os"
)

func getTempFolder() (string, error) {
	return os.MkdirTemp("", "boilerplate-cache*")
}
