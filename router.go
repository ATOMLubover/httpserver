// router
package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Router controlls the all routes
// and provides API to register routes and handle requests
type _Router struct {
	RouteGroup // inherit RouteGroup

	httpctxTimeoutTime time.Duration // timeout time for getting Context
	processTimeoutTime time.Duration // timeout time for handler

	contextPool *_ContextPool // context pool
}

// Implement http.Handler interface.
func (r *_Router) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	// 1. Find the route node which can handle the request.
	method := stringToMethod[request.Method]
	node, uriParams := r.routeTree._Search(request.URL.Path, method)
	if node == nil {
		gLogger.Debug(fmt.Sprintf("Invalid request URI: %s, method: %s", request.URL.Path, request.Method))
		// Return 400 if server cannot handle the request.
		http.Error(writer, "Invalid request URI.", http.StatusNotFound)
		return
	}

	// 2. If a matching route node is found, assemble the final handler.
	finalHandler := node.handlers[method]
	middlewareChain := node.group._GetMiddlewareChain()
	for i := len(middlewareChain) - 1; i >= 0; i-- {
		finalHandler = middlewareChain[i](finalHandler)
	}

	// 3. Try getting a usable context from context pool to handle the request.
	// 	  _Get function will blocks the goroutine, whose time depends on ctxTimeoutTime.
	ctx, err := r.contextPool._Get()
	if err != nil {
		// If the context pool is empty, just return 503.
		http.Error(writer, "Server busy, try later.", http.StatusServiceUnavailable)
		return
	}

	// Ensure that the ctx will be returned to contextPool
	defer r.contextPool._Return(ctx)
	// Prepare ctx after getting.
	ctx._Update(request, writer)
	ctx.uriParams = uriParams

	// Create handler timeout context
	timeoutCtx, processCancel := context.WithTimeout(context.Background(), r.processTimeoutTime)
	defer processCancel()

	// Conditional variable which synchronizes the processing.
	handlerDone := make(chan struct{})
	// Handle in another goroutine in order to apply timeout.
	go func() {
		// Exception catcher.
		defer func() {
			if r := recover(); r != nil {
				ctx.errHandle = fmt.Errorf("panic when handling: %v", r)
			}

			// Notify the caller goroutine to go on.
			close(handlerDone)
		}()

		// We do not support to abort a handler when it is running at current stage,
		// so we just test whether the response has been sent before step into it.
		// After handler finishes, check whether the response has been sent again in case it has been timeout.
		// Though there is some performance waste.
		if !ctx._IsResponseSent() {
			finalHandler(ctx)
		}
	}()

	// Wait for the timeout or the handler to finish.
	select {
	case <-handlerDone:
		// If the response has been sent, just return.
		if !ctx._CasIsResponseSent() {
			return
		}

		// If the handler panicked, return 500.
		if ctx.errHandle != nil {
			http.Error(writer, "Error occurred when handling.", http.StatusInternalServerError)

			gLogger.Debug(fmt.Sprintf("Error occurred with: %v, route: %s", ctx.errHandle, node.pattern))
			return
		}
		// Send normal response when no error.
		ctx._Send()

	case <-timeoutCtx.Done():
		// If the response has been sent, just return
		if !ctx._CasIsResponseSent() {
			return
		}

		http.Error(writer, "Timeout when handling.", http.StatusGatewayTimeout)

		gLogger.Debug(fmt.Sprintf("Timeout with route: %s", node.pattern))
	}

	// // 3. Get a timeout context in case blocking request when no context available.
	// httpctxTimeoutCtx, httpctxCancel := context.WithTimeout(context.Background(), r.httpctxTimeoutTime)
	// defer httpctxCancel()

	// // 4. Try getting a usable context from context pool to handle the request.
	// // 	  Or we will receive a timeout signal and just return an error response.
	// select {
	// case ctx := <-r.contextPool._GetRawPool():
	// 	{
	// 		// Ensure that the ctx will be returned to contextPool
	// 		defer r.contextPool._Return(ctx)
	// 		// Prepare ctx after getting.
	// 		ctx._Update(request, writer)
	// 		ctx.uriParams = uriParams

	// 		// Create handler timeout context
	// 		processTimeoutCtx, processCancel := context.WithTimeout(context.Background(), r.processTimeoutTime)
	// 		defer processCancel()

	// 		// Conditional variable which synchronizes the processing.
	// 		handlerDone := make(chan struct{})
	// 		// Handle in another goroutine in order to apply timeout.
	// 		go func() {
	// 			defer func() {
	// 				if r := recover(); r != nil {
	// 					ctx.errHandle = fmt.Errorf("panic with context(%d): %v", ctx.id, r)
	// 				}

	// 				// Notify the caller goroutine to go on.
	// 				close(handlerDone)
	// 			}()

	// 			// We do not support to abort a handler when it is running at current stage,
	// 			// so we just test whether the response has been sent before step into it.
	// 			// After handler finishes, check whether the response has been sent again in case it has been timeout.
	// 			// Though there is some performance waste.
	// 			if !ctx._IsResponseSent() {
	// 				finalHandler(ctx)
	// 			}
	// 		}()

	// 		// Wait for the timeout or the handler to finish.
	// 		select {
	// 		case <-handlerDone:
	// 			// If the response has been sent, just return
	// 			if !ctx._CasIsResponseSent() {
	// 				return
	// 			}

	// 			// Cancel processTimeoutCtx in case of resource leakage.
	// 			processCancel()

	// 			// If the handler panicked, return 500.
	// 			if ctx.errHandle != nil {
	// 				http.Error(writer, "Error occurred when handling.", http.StatusInternalServerError)

	// 				gLogger.Debug(fmt.Sprintf("Error occurred with context(%d): %v, route: %s", ctx.id, ctx.errHandle, node.pattern))
	// 				return
	// 			}
	// 			// Send normal response when no error.
	// 			ctx._Send()

	// 		case <-processTimeoutCtx.Done():
	// 			// If the response has been sent, just return
	// 			if !ctx._CasIsResponseSent() {
	// 				return
	// 			}

	// 			http.Error(writer, "Timeout when handling.", http.StatusGatewayTimeout)

	// 			gLogger.Debug(fmt.Sprintf("Timeout with context(%d), route: %s", ctx.id, node.pattern))
	// 		}
	// 	}

	// case <-httpctxTimeoutCtx.Done():
	// 	// If the context pool is empty, just return 503.
	// 	http.Error(writer, "Server busy, try later.", http.StatusServiceUnavailable)
	// }
}

// Create a new router.
func _NewRouter() *_Router {
	return &_Router{
		RouteGroup: RouteGroup{
			routeTree:       _NewRouteTree(),
			middlewareCache: _NewMiddlewareChainCache(), // 10 now because there are not many route groups

			parent:      nil,
			children:    make([]*RouteGroup, 0),
			prefix:      "/",
			middlewares: make([]MiddlewareFunc, 0),
		},

		httpctxTimeoutTime: time.Second * 3, // wait 3sec when trying getting Context
		processTimeoutTime: time.Second * 5, // handler will have 5sec to process after getting a context

		contextPool: _NewContextPool(),
	}
}

// Add a route to router
func (r *_Router) AddRoute(method Method, pattern string, handler HandlerFunc) {
	if !_CheckPatternCharactors(pattern) {
		panic("invalid URI pattern format: " + pattern)
	}

	// process root route specially
	if pattern == "/" {
		r.routeTree.root.handlers[method] = handler
		r.routeTree.root.pattern = pattern
		r.routeTree.root.isLeaf = true

		gLogger.Info(fmt.Sprintf("Added root in router: %s, method: %s", pattern, method))

		return
	}

	// splice route pattern into parts in order to insert into route tree
	parts, err := _TransformPatternIntoParts(pattern)
	if err != nil || len(parts) == 0 {
		panic(err.Error())
	}

	// insert into route tree
	r.routeTree._Insert(method, pattern, &r.RouteGroup, handler)

	gLogger.Info(fmt.Sprintf("Route added in router: %s [%s]", pattern, method))
}

// Add a new child route group into this route group.
// Return the child route group newly created.
func (r *_Router) AddGroup(prefix string) *RouteGroup {
	// remove the last '/'
	parentPrefix := r.prefix[:len(r.prefix)-1]
	// create child route group
	child := _NewRouteGroup(r.middlewareCache, &r.RouteGroup, parentPrefix+prefix)

	// add child route group to children
	r.children = append(r.children, child)

	return child
}

// // Use a new middleware into router
// func (r *_Router) UseMiddleware(middleware ...MiddlewareFunc) {
// 	r.middlewares = append(r.middlewares, middleware...)
// }

// // Get entire middleware chain of current route group.
// func (r *_Router) _GetMiddlewareChain() []MiddlewareFunc {
// 	// return middlewares of router directly, because router does not have parent
// 	return r.middlewares
// }

// // Find the target route node to handle a request
// func (r *_Router) _FindNode(uri string, method Method) (*_RouteNode, map[string]string) {
// 	return r.routeTree._Search(uri, method)
// }
