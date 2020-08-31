// +build aix darwin dragonfly freebsd js,wasm linux netbsd openbsd solaris

package getter_helper

import (
	"io/ioutil"
)

func getTempFolder() (string, error) {
	return ioutil.TempDir("", "boilerplate-cache*")
}
