// some useful utility functions
package httpserver

import (
	"regexp"
)

func CheckValidUri(uri string) bool {
	pattern := `^[a-zA-Z0-9\-._~!$&'()*+,;=:@/%]+$`
	matched, _ := regexp.MatchString(pattern, uri)
	return matched
}
