// router
package httpserver

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"unicode"
)

// router controlls the all routes
// and provides interfaces to register routes and handle requests
type _Router struct {
	routeTree *_RouteTree            // ref of route tree
	handlers  map[string]HandlerFunc // references to handler
}

// implement http.Handler interface
func (r *_Router) _ServeHttpImpl(w http.ResponseWriter, req *http.Request) {

}

// create a new router
func _NewRouter() *_Router {
	return &_Router{
		routeTree: _NewRouteTree(),
		handlers:  make(map[string]HandlerFunc),
	}
}

// validate colon wildcard rules
func _ValidateColonWildcard(part string) error {
	// format should be like ":name"
	if len(part) == 1 {
		return errors.New("colon wildcard must be named")
	}
	// colon should not be repeated
	if strings.Count(part, ":") > 1 {
		return errors.New("too many ':' in part: " + part)
	}
	// wildcard naming should be valid
	if !_CheckColonNameValid(strings.TrimPrefix(part, ":")) {
		return errors.New("invalid wildcard name: " + part)
	}

	return nil
}

// validate asterisk wildcard rules
func _ValidateAsteriskWildcard(part string, remainingPartsNum int) error {
	// make sure asterisk is last part
	if remainingPartsNum >= 1 {
		return errors.New("asterisk must be last part")
	}

	// validate naming
	name := strings.TrimPrefix(part, "*")
	if !_CheckAsteriskNameValid(name) {
		return errors.New("invalid asterisk name: " + name)
	}

	return nil
}

// check colon naming validation
func _CheckColonNameValid(name string) bool {
	if name == "" {
		return false
	}

	// first char must be letter or _
	if !(unicode.IsLetter(rune(name[0])) || name[0] == '_') {
		return false
	}

	for _, r := range name {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return false
		}
	}

	// return true after checking
	return true
}

// check asterisk naming validation
func _CheckAsteriskNameValid(name string) bool {
	// asterisk allows empty name
	if name == "" {
		return true
	}

	// first char must be letter or _
	if !(unicode.IsLetter(rune(name[0])) || name[0] == '_') {
		return false
	}

	for _, r := range name {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return false
		}
	}

	// return true after checking
	return true
}

func (r *_Router) _AddRoute(method Method, pattern string, handler HandlerFunc) {
	if !CheckValidPattern(pattern) {
		// slog.Warn("URI pattern format: " + pattern)
		// return
		panic("URI pattern format: " + pattern)
	}

	// process root route specially
	if pattern == "/" {
		r.routeTree.root.handlers[method] = handler
		r.routeTree.root.pattern = pattern
		r.routeTree.root.isLeaf = true

		key := method.String() + "-" + pattern
		r.handlers[key] = handler

		slog.Info(fmt.Sprintf("Added root in router: %s, method: %s", pattern, method))

		return
	}

	// splice route pattern into parts in order to insert into route tree
	parts, err := _TransformPatternIntoParts(pattern)
	if err != nil || len(parts) == 0 {
		//slog.Warn(fmt.Sprintf("Invalid route pattern: %s, error: %s", pattern, err.Error()))
		//return
		panic(err.Error())
	}

	// insert into route tree
	r.routeTree._Insert(method, pattern, handler)
	// record handlers into router
	key := method.String() + "-" + pattern
	r.handlers[key] = handler

	slog.Info(fmt.Sprintf("Added route in router: %s", pattern))
}
