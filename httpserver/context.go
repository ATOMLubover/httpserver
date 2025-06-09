package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
)

// Send type.
const (
	NORMAL_CONT int = iota
	STREAM_STATIC
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

	sendType int // 0: short text response, 1: file stream

	bodyBuffer *bytes.Buffer // temporarily store the body of short response
	filepath   string        // file physical path
	filename   string        // complete file name

	errHandle  error // the error collected when handling
	statusCode int   // the status code to write in the response

	isResponseSent uint32 // atomic flag, 0 for false, 1 for true
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

		sendType: 0,

		bodyBuffer: bytes.NewBuffer(nil),
		filename:   "",
		filepath:   "",

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
	// Keep the original map not deconstructed.
	for key := range c.queryParams {
		delete(c.queryParams, key)
	}

	c.sendType = 0

	c.bodyBuffer.Reset()
	c.filename = ""
	c.filepath = ""

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

// Get header of request.
func (c *Context) GetReqHeader(key string) string {
	return c.rawRequest.Header.Get(key)
}

// Return text/plain response
func (c *Context) Text(statusCode int, msg string) {
	c.rawResponseWriter.Header().Set("Content-Type", "text/plain")
	c.bodyBuffer.WriteString(msg)

	c.statusCode = statusCode
}

// Return JSON response.
func (c *Context) Json(statusCode int, obj interface{}) {
	c.rawResponseWriter.Header().Set("Content-Type", "application/json")
	json.NewEncoder(c.bodyBuffer).Encode(obj)

	c.statusCode = statusCode
}

// Return file response using stream.
// But it is not recommanded that use this function to handle frequent file request.
func (c *Context) StreamFile(statusCode int, filePath string) {
	filePath = filepath.Clean(filePath)
	filePathParts := strings.Split(filePath, string(filepath.Separator))
	c.filename = filePathParts[len(filePathParts)-1]
	c.filepath = filePath

	c.sendType = 1
}

// Get the body of request.
func (c *Context) GetBody() ([]byte, error) {
	if c.rawRequest.Body == nil {
		return nil, errors.New("no request body")
	}
	defer c.rawRequest.Body.Close()

	return io.ReadAll(c.rawRequest.Body)
}

// Set the header of response.
// Notice that an existing header with the same key will not overwritten.
func (c *Context) SetHeader(key, value string) {
	c.rawResponseWriter.Header().Set(key, value)
}

// Modify existing header.
func (c *Context) ModifyHeader(key string, values []string) {
	c.rawResponseWriter.Header()[key] = values
}

// Send response.
// We always send header first and then send the body.
func (c *Context) _Send() {
	switch c.sendType {
	case 0:
		{
			if c.bodyBuffer == nil {
				gLogger.Error(fmt.Sprintf("Body buffer is nil when sending response(context: %d, URI: %s).", c.id, c.rawRequest.RequestURI))
				return
			}

			// Send header.
			c.rawResponseWriter.WriteHeader(c.statusCode)

			// Send body.
			c.bodyBuffer.WriteTo(c.rawResponseWriter)
		}

	case 1:
		{
			if c.filepath == "" || c.filename == "" {
				http.Error(c.rawResponseWriter, "File response failed.", http.StatusInternalServerError)

				gLogger.Warn(fmt.Sprintf("File path is %s file name is %s when sending response(context: %d, URI: %s).", c.filepath, c.filename, c.id, c.rawRequest.RequestURI))
				return
			}

			// Open file.
			file, err := os.Open(c.filepath)
			defer file.Close()
			if err != nil {
				http.Error(c.rawResponseWriter, "File response failed.", http.StatusInternalServerError)

				gLogger.Error(fmt.Sprintf("Failed to open file(%s) when sending response(context: %d, URI: %s).", c.filepath, c.id, c.rawRequest.RequestURI))
				return
			}

			// Get meta.
			meta, err := file.Stat()
			if err != nil {
				http.Error(c.rawResponseWriter, "File response failed.", http.StatusInternalServerError)

				gLogger.Error(fmt.Sprintf("Failed to get meta of file(%s) when sending response(context: %d, URI: %s).", c.filepath, c.id, c.rawRequest.RequestURI))
				return
			}

			// Set content length.
			fileSize := meta.Size()
			c.SetHeader("Content-Length", strconv.FormatInt(fileSize, 10))

			contentType := mime.TypeByExtension(filepath.Ext(c.filepath))
			// If contentType is "", just set it to "application/octet-stream".
			if contentType == "" {
				contentType = "application/octet-stream"
			}

			// Send header.
			c.rawResponseWriter.WriteHeader(c.statusCode)

			// Send file body using stream.
			// Because we set Content-Length, chunked will disabled.
			// But this function still apply 0-copy transferring.
			io.Copy(c.rawResponseWriter, file)
		}
	}
}
