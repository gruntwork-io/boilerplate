// Package logging provides a shared application logger.
package logging

import (
	"log"
	"os"
)

// Logger is a simple logger we can use to get consistent log formatting throughout the app
var Logger = log.New(os.Stdout, "[boilerplate] ", log.LstdFlags)
