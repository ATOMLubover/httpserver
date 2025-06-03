// some useful utility functions
package httpserver

import (
	"regexp"
)

func CheckValidPattern(uri string) bool {
	pattern := `^[a-zA-Z0-9_/:*]+$`
	matched, _ := regexp.MatchString(pattern, uri)
	return matched
}
