package cache

import (
	"net/http"
	"time"

	"github.com/ATOMLubover/httpserver/internal/context"
	"github.com/ATOMLubover/httpserver/internal/util"
)

// The data record of a GET response.
type GetResponseData struct {
	StatusCode int

	SendType context.ResponseSendType

	Headers http.Header
	Body    []byte
}

// Cache for GET responses.
type GetResponseCache struct {
	util.LruCache[GetResponseData]
}

// Create a new GET response cache.
func NewGetResponseCache(maxSize int, rebuildInterval time.Duration) *GetResponseCache {
	return &GetResponseCache{
		LruCache: *util.NewLruCache[GetResponseData](maxSize, rebuildInterval),
	}
}
