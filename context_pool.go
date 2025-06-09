package httpserver

// Context pool with fixed size.
// Store contexts waiting to be reused.
type _ContextPool struct {
	pool chan *Context // buffered pool to get and return contexts
	size int           // size of buffer
}

// Create a new context pool.
func _NewContextPool() *_ContextPool {
	p := &_ContextPool{
		pool: make(chan *Context, gConfig.routerCfg.ctxPoolSize),
	}

	// Initialize contexts.
	for i := 0; i < gConfig.routerCfg.ctxPoolSize; i++ {
		p.pool <- _NewContext(i)
	}
	p.size = gConfig.routerCfg.ctxPoolSize

	return p
}

// // Try getting a context from the pool.
// // Will be blocked if there are no contexts available.
// func (p *_ContextPool) _Get() *Context {
// 	// Blocking may happens here.
// 	return <-p.pool
// }

// Get the raw pool channel.
func (p *_ContextPool) _GetRawPool() chan *Context {
	return p.pool
}

// Return a context back to the pool.
// The context return will be automatically reset.
func (p *_ContextPool) _Return(ctx *Context) {
	// Reset before returning in case of concurrency problems.
	ctx._Reset()

	select {
	case p.pool <- ctx:
		// Put the context back to the pool.
	default:
		// Or the pool is full.But it is impossible.
		gLogger.Warn("Context pool is full unexpectedly.")
	}
}
