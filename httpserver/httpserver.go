// implamentations
package httpserver

// http methods
type Method int

// http methods enumerations
const (
	GET Method = iota
	POST
	PUT
	DELETE
)

var methodToString = map[Method]string{
	GET:    "GET",
	POST:   "POST",
	PUT:    "PUT",
	DELETE: "DELETE",
}

func (m Method) String() string {
	return methodToString[m]
}

// alias of functions
type (
	// handler function
	HandlerFunc func(ctx *Context)

	// middleware function
	MiddlewareFunc func(next HandlerFunc) HandlerFunc
)
