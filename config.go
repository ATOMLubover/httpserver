package httpserver

import (
	"log/slog"
	"time"
)

// Total config.
type _Config struct {
	port     uint16
	logLevel slog.Level

	routerCfg        _RouterConfig
	midChainCacheCfg _MiddlewareChainCacheConfig
}

// Router config.
type _RouterConfig struct {
	ctxPoolSize            int
	ctxSubPoolNum          int
	ctxTimeoutTime         time.Duration
	subpoolCleanupInterval time.Duration
	subpoolDropTime        time.Duration

	processTimeoutTime time.Duration
}

// Middleware chain cache config.
type _MiddlewareChainCacheConfig struct {
	maxSize         int
	rebuildInterval time.Duration
}

// Instance of gConfig.
// Default values in.
var gConfig *_Config = &_Config{
	port:     8080,
	logLevel: slog.LevelInfo,

	routerCfg: _RouterConfig{
		ctxPoolSize:            20,
		ctxSubPoolNum:          5,
		ctxTimeoutTime:         2 * time.Second,
		subpoolCleanupInterval: 5 * time.Second,
		subpoolDropTime:        10 * time.Second,

		processTimeoutTime: 10 * time.Second,
	},

	midChainCacheCfg: _MiddlewareChainCacheConfig{
		maxSize:         10,
		rebuildInterval: 1 * time.Second,
	},
}

// Set the config.
func ModifyConfig(opts ...ConfigOption) {
	for _, opt := range opts {
		opt(gConfig)
	}
}

// Configuration options.
type ConfigOption func(c *_Config)

// Set server port.
func WithPort(port uint16) ConfigOption {
	return func(c *_Config) {
		c.port = port
	}
}

// Set log level.
func WithLogLevel(level slog.Level) ConfigOption {
	return func(c *_Config) {
		c.logLevel = level
	}
}

// Set default context pool size.
func WithContextPoolSize(size int) ConfigOption {
	return func(c *_Config) {
		c.routerCfg.ctxPoolSize = size
	}
}

// Set timeout time when getting a context from pool.
// Timeout will lead to a 504 response.
func WithContextPoolTimeout(timeout time.Duration) ConfigOption {
	return func(c *_Config) {
		c.routerCfg.ctxTimeoutTime = timeout
	}
}

// Set the number of context sub pools.
// Sub pools will contains the (2^(count-1)-2)*size contexts every one.
func WithContextSubPoolCount(count int) ConfigOption {
	return func(c *_Config) {
		c.routerCfg.ctxSubPoolNum = count
	}
}

// Set subpool of context pool cleanup interval.
func WithContextSubPoolCleanupInterval(interval time.Duration) ConfigOption {
	return func(c *_Config) {
		c.routerCfg.subpoolCleanupInterval = interval
	}
}

// Set subpool of context pool drop time limit.
// A subpool which has not been used for duration will be dropped.
func WithContextSubPoolDropTime(duration time.Duration) ConfigOption {
	return func(c *_Config) {
		c.routerCfg.subpoolDropTime = duration
	}
}

// Set timeout time when handling a requset.
// Timeout will lead to a 503 response.
func WithHandleTimeout(timeout time.Duration) ConfigOption {
	return func(c *_Config) {
		c.routerCfg.processTimeoutTime = timeout
	}
}

// Set the size of the middleware chain cache.
func WithMiddlewareChainCacheSize(size int) ConfigOption {
	return func(c *_Config) {
		c.midChainCacheCfg.maxSize = size
	}
}

// Set the LRU interval of the middleware chain cache.
func WithMiddlewareChainCacheLruInterval(interval time.Duration) ConfigOption {
	return func(c *_Config) {
		c.midChainCacheCfg.rebuildInterval = interval
	}
}
