package httpserver

// HTTP request
type Request struct {
	Method  Method
    
	Url     string
	Uri     string
	Query   map[string]string
	Section string
}

// HTTP response
type Response struct {
}

// HTTP context
type Context struct {
	Request  Request
	Response Response
}
