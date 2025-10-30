package pslog

import "os"

var exitProcess = func() {
	os.Exit(1)
}
