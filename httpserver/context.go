package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
)

// HTTP context
type Context struct {
	id int // the identifier of the context in pool

	rawRequest        *http.Request
	rawResponseWriter http.ResponseWriter

	uriParams   map[string]string      // params gotton when parsing URI
	queryParams map[string]string      // params of GET query
	section     string                 // param of the hash section
	userValues  map[string]interface{} // values users set

	errHandle  error // the error collected when handling
	statusCode int   // the status code to write in the response

	isResponseSent uint32 // atomic flag, 0 for false, 1 for true

	// isAborted bool // whether abort the middlewares after handler(depends on the implementation of MiddlewareFunc)
}

// Create a new context.
func _NewContext(id int) *Context {
	ctx := &Context{
		id: id,

		rawRequest:        nil,
		rawResponseWriter: nil,

		uriParams:   nil,
		queryParams: nil,
		section:     "",
		userValues:  make(map[string]interface{}), // only this member is not nil at first

		errHandle:  nil,
		statusCode: http.StatusOK, // default as 200

		isResponseSent: 0,
	}

	return ctx
}

// Update the context info when be used.
func (c *Context) _Update(rawRequest *http.Request, rawResponseWriter http.ResponseWriter) {
	c.rawRequest = rawRequest
	c.rawResponseWriter = rawResponseWriter
}

// Reset a context when it is returned to the pool.
func (c *Context) _Reset() {
	c.rawRequest = nil
	c.rawResponseWriter = nil

	c.uriParams = nil
	c.queryParams = nil
	c.section = ""
	c.userValues = nil

	c.errHandle = nil
	c.statusCode = http.StatusOK

	c.isResponseSent = 0
}

// Check whether response has been sent.
func (c *Context) _IsResponseSent() bool {
	return atomic.LoadUint32(&c.isResponseSent) == 1
}

// Check whether is it able to send a response.
// Will set isResponseSent flag to true implicitly when it returns true.
func (c *Context) _CasIsResponseSent() bool {
	return atomic.CompareAndSwapUint32(&c.isResponseSent, 0, 1)
}

// Get the URI path parameters.
func (c *Context) GetUriParams() map[string]string {
	return c.uriParams
}

// Get GET query parameters.
func (c *Context) GetQueryParams() map[string]string {
	// Quickly return stored params.
	if c.queryParams != nil {
		return c.queryParams
	}

	for key, value := range c.rawRequest.URL.Query() {
		c.queryParams[key] = value[0]
	}

	return c.queryParams
}

// Store KV of user's.
func (c *Context) SetKeyValue(key string, value interface{}) {
	c.userValues[key] = value
}

// Get KV of user's.
func (c *Context) GetKeyValue(key string) (interface{}, bool) {
	value, isExisating := c.userValues[key]
	return value, isExisating
}

// Return text/plain response
func (c *Context) Text(statusCode int, msg string) {
	c.rawResponseWriter.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(c.rawResponseWriter, msg)

	c.statusCode = statusCode
}

// Return JSON response.
func (c *Context) Json(statusCode int, obj interface{}) {
	c.rawResponseWriter.Header().Set("Content-Type", "application/json")
	json.NewEncoder(c.rawResponseWriter).Encode(obj)

	c.statusCode = statusCode
}

// Get the body of response.
func (c *Context) GetBody() ([]byte, error) {
	if c.rawRequest.Body == nil {
		return nil, errors.New("no request body")
	}
	defer c.rawRequest.Body.Close()

	return io.ReadAll(c.rawRequest.Body)
}

// Set the header of response.
func (c *Context) SetHeader(key, value string) {
	c.rawResponseWriter.Header().Set(key, value)
}
