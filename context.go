package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// Send type.
const (
	NORMAL_CONT int = iota
	STREAM_STATIC
)

// Multipart max initial read size(1MB).
const PART_MAX_INIT_READ_SIZE = 1 << 20

// Multipart max total read size(10MB).
const PART_MAX_TOTAL_READ_SIZE = 10 << 20

// Upload file tmp relative dir.
const UPLOAD_TMP_REL_DIR = "/tmp/upload/"

// Request form file meta.
type FormFileMeta struct {
	FieldName string // field name
	FileName  string // file name
	FileSize  int64  // file size

	contentType string // content type of file

	tempPath string // temp file path
	isTemp   bool   // whether the file is stored as temp file

	memFile  *bytes.Buffer // meta of memory file
	tempFile *os.File      // meta of temp file
}

// Request data.
type _RequestData struct {
	uriParams   map[string]string // params gotton when parsing URI
	queryParams map[string]string // params of GET query
	section     string            // param of the hash section

	formValues map[string]string        // values of form
	formFiles  map[string]*FormFileMeta // files of form

	userValues map[string]interface{} // values users set
}

// Response data.
type _ResponseData struct {
	sendType int // 0: short text response, 1: file stream

	bodyBuffer *bytes.Buffer // temporarily store the body of short response
	filepath   string        // file physical path

	statusCode int // status code
}

// HTTP context
type Context struct {
	rawRequest        *http.Request
	rawResponseWriter http.ResponseWriter

	reqData *_RequestData
	resData *_ResponseData

	errHandle error // the error collected when handling

	isFormParsed   bool   // whether the form data has been parsed
	isResponseSent uint32 // atomic flag, 0 for false, 1 for true
}

// Create a new context.
func _NewContext() *Context {
	ctx := &Context{
		rawRequest:        nil,
		rawResponseWriter: nil,

		reqData: &_RequestData{
			uriParams:   make(map[string]string),
			queryParams: make(map[string]string),
			section:     "",

			userValues: make(map[string]interface{}),

			formValues: make(map[string]string),
			formFiles:  make(map[string]*FormFileMeta),
		},

		resData: &_ResponseData{
			sendType: 0,

			bodyBuffer: bytes.NewBuffer(nil),
			filepath:   "",

			statusCode: http.StatusOK, // default as 200
		},

		errHandle: nil,

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

	// Reset the request data.
	{
		for key := range c.reqData.uriParams {
			delete(c.reqData.uriParams, key)
		}
		for key := range c.reqData.queryParams {
			delete(c.reqData.queryParams, key)
		}
		c.reqData.section = ""

		for key := range c.reqData.formValues {
			delete(c.reqData.formValues, key)
		}
		for key, file := range c.reqData.formFiles {
			if file.isTemp {
				file.tempFile.Close()
				os.Remove(file.tempPath)
			}

			delete(c.reqData.formFiles, key)
		}
	}

	// Reset the response data.
	{
		c.resData.sendType = 0

		c.resData.bodyBuffer.Reset()
		c.resData.filepath = ""

		c.resData.statusCode = http.StatusOK
	}

	c.errHandle = nil

	c.isResponseSent = 0
	c.isFormParsed = false
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
func (c *Context) ReqUriParams() map[string]string {
	return c.reqData.uriParams
}

// Get GET query parameters.
func (c *Context) ReqQueryParams() map[string]string {
	// Quickly return stored params.
	if c.reqData.queryParams != nil {
		return c.reqData.queryParams
	}

	for key, value := range c.rawRequest.URL.Query() {
		c.reqData.queryParams[key] = value[0]
	}

	return c.reqData.queryParams
}

// Get header of request.
func (c *Context) ReqHeader(key string) string {
	return c.rawRequest.Header.Get(key)
}

// Store KV of user's.
func (c *Context) SaveKeyValue(key string, value interface{}) {
	c.reqData.userValues[key] = value
}

// Get KV of user's.
func (c *Context) LoadKeyValue(key string) (interface{}, bool) {
	value, isExisating := c.reqData.userValues[key]
	return value, isExisating
}

// Return text/plain response
func (c *Context) Text(statusCode int, msg string) {
	c.rawResponseWriter.Header().Set("Content-Type", "text/plain")
	c.resData.bodyBuffer.WriteString(msg)

	c.resData.statusCode = statusCode
}

// Return JSON response.
func (c *Context) Json(statusCode int, obj interface{}) {
	c.rawResponseWriter.Header().Set("Content-Type", "application/json")
	json.NewEncoder(c.resData.bodyBuffer).Encode(obj)

	c.resData.statusCode = statusCode
}

// Return file response using stream.
// But it is not recommanded that use this function to handle frequent file request.
func (c *Context) File(statusCode int, filePath string) {
	filePath = filepath.Clean(filePath)
	c.resData.filepath = filePath

	c.resData.sendType = 1
}

// Set the header of response.
// Notice that an existing header with the same key will not overwritten.
func (c *Context) AppendHeader(key, value string) {
	c.rawResponseWriter.Header().Add(key, value)
}

// Modify existing header.
func (c *Context) ModifyHeader(key string, values ...string) {
	c.rawResponseWriter.Header()[key] = values
}

// Send response.
// We always send header first and then send the body.
func (c *Context) _Send() {
	switch c.resData.sendType {
	case 0:
		{
			if c.resData.bodyBuffer == nil {
				gLogger.Error(fmt.Sprintf("Body buffer is nil when sending response(URI: %s).", c.rawRequest.RequestURI))
				return
			}

			// Send header.
			c.rawResponseWriter.WriteHeader(c.resData.statusCode)

			// Send body.
			c.resData.bodyBuffer.WriteTo(c.rawResponseWriter)
		}

	case 1:
		{
			if c.resData.filepath == "" {
				http.Error(c.rawResponseWriter, "File response failed.", http.StatusInternalServerError)

				gLogger.Warn(fmt.Sprintf("File(path: %s) failed when sending response(URI: %s).", c.resData.filepath, c.rawRequest.RequestURI))
				return
			}

			c.resData.filepath = filepath.Clean(c.resData.filepath)
			// Use 0-copy sendfile to minimize the performance waste.
			http.ServeFile(c.rawResponseWriter, c.rawRequest, c.resData.filepath)
		}
	}
}

// Preparse the multipart form data.
func (c *Context) _ParseMultipartFormData() error {
	// Quick return.
	if c.isFormParsed {
		return nil
	}

	// Check the content type.
	if !strings.HasPrefix(c.rawRequest.Header.Get("Content-Type"), "multipart/form-data") {
		return errors.New("invalid content type for multipart form data")
	}

	// Process the body according to the transfer-encoding.
	return c._ProcessMultipart()
}

// Use multipart reader to parse the form data.
func (c *Context) _ProcessMultipart() error {
	// Parse content-type.
	_, params, err := mime.ParseMediaType(c.rawRequest.Header.Get("Content-Type"))
	if err != nil {
		return fmt.Errorf("invalid content type for multipart form data: %w", err)
	}

	// Get boundary.
	boundary := params["boundary"]
	if boundary == "" {
		return errors.New("invalid multipart/form-data boundary")
	}

	mulreader := multipart.NewReader(c.rawRequest.Body, boundary)

	handlePart := func(part *multipart.Part) error {
		// Get the field name.
		fieldName := part.FormName()

		// Skip empty field names.
		if fieldName == "" {
			return nil
		}

		switch part.FileName() {
		case "": // Regular form field(key value).
			buf := bytes.NewBuffer(nil)

			if _, err := io.Copy(buf, part); err != nil {
				return fmt.Errorf("error when parsing kv multipart form data: %w", err)
			}

			c.reqData.formValues[fieldName] = buf.String()
			return nil

		default: // File.
			return c._ProcessFilePart(part)
		}
	}

	// Loop thru every part.
	for {
		part, err := mulreader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error when parsing multipart form data: %w", err)
		}

		err = handlePart(part)
		if err != nil {
			part.Close()
			return fmt.Errorf("error when parsing multipart form data: %w", err)
		}

		part.Close()
	}

	c.isFormParsed = true

	return nil
}

// Process file.
func (c *Context) _ProcessFilePart(part *multipart.Part) error {
	fieldname := part.FormName()
	filename := filepath.Base(part.FileName())

	fileMeta := &FormFileMeta{
		FieldName: fieldname,
		FileName:  filename,

		contentType: part.Header.Get("Content-Type"),

		memFile: bytes.NewBuffer(nil),
	}

	// Read initial data.
	n, err := io.CopyN(fileMeta.memFile, part, PART_MAX_INIT_READ_SIZE)
	if err != nil && err != io.EOF {
		return fmt.Errorf("error when reading initial data: %w", err)
	}

	if n < PART_MAX_INIT_READ_SIZE && err == io.EOF {
		// Save directly in memory.
		fileMeta.isTemp = false
		fileMeta.FileSize = n

		c.reqData.formFiles[fieldname] = fileMeta
		return nil
	}

	// Save large files on disk.
	return c._HanldleLargeFileSaving(part, fileMeta)
}

// Save the uploaded file into memory.
func (c *Context) _HanldleLargeFileSaving(part *multipart.Part, meta *FormFileMeta) error {
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	uploadDir := filepath.Join(exeDir, UPLOAD_TMP_REL_DIR)

	// Ensure that tmp dir exists.
	if err := os.MkdirAll(uploadDir, 0700); err != nil {
		return fmt.Errorf("failed to create upload dir: %w", err)
	}

	// Create a tmp file.
	file, err := os.CreateTemp(uploadDir, "upload-*.tmp")
	if err != nil {
		return fmt.Errorf("error when creating temp file: %w", err)
	}

	// Copy the existing memory content into file.
	buf := bytes.NewBuffer(nil)
	if _, err := io.CopyBuffer(file, meta.memFile, buf.Bytes()); err != nil {
		return fmt.Errorf("error when copying buf data to temp file: %w", err)
	}
	meta.memFile.Reset()

	// Limit the total size of file on disk.
	lmtreader := io.LimitReader(part, PART_MAX_TOTAL_READ_SIZE)

	// Read rest of part into file.
	buf.Reset()
	if _, err := io.CopyBuffer(file, lmtreader, buf.Bytes()); err != nil {
		return fmt.Errorf("error when copying part data to temp file: %w", err)
	}

	// Get stat of file.
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("error when getting stat of temp file: %w", err)
	}

	meta.isTemp = true
	meta.tempFile = file
	meta.tempPath = file.Name()
	meta.FileSize = stat.Size()

	fieldname := part.FormName()
	c.reqData.formFiles[fieldname] = meta
	return nil
}

// Get request form values.
func (c *Context) FormValues(field string) (string, bool) {
	if err := c._ParseMultipartFormData(); err != nil {
		c.errHandle = err
		return "", false
	}

	values, existing := c.reqData.formValues[field]
	return values, existing
}

// Get request form files(meta only).
func (c *Context) FormFile(key string) (*FormFileMeta, bool) {
	if err := c._ParseMultipartFormData(); err != nil {
		c.errHandle = err
		return nil, false
	}

	values, existing := c.reqData.formFiles[key]
	return values, existing
}
