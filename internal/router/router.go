package router

import (
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"
)

type Middleware func(http.Handler) http.Handler

type Router struct {
	mux         *http.ServeMux
	base        string
	middlewares []Middleware
	parent      *Router
}

func New() *Router {
	return &Router{
		mux:  http.NewServeMux(),
		base: "",
	}
}

func (r *Router) Use(mw ...Middleware) {
	r.middlewares = append(r.middlewares, mw...)
}

func (r *Router) With(mw ...Middleware) *Router {
	cp := *r
	cp.middlewares = append(append([]Middleware{}, r.middlewares...), mw...)
	return &cp
}

func (r *Router) Group(prefix string) *Router {
	p := &Router{
		mux:         r.mux,
		base:        joinPaths(r.base, prefix),
		middlewares: append([]Middleware{}, r.middlewares...), // наследуем стек
		parent:      r,
	}
	return p
}

func (r *Router) Route(prefix string, fn func(*Router)) {
	child := r.Group(prefix)
	fn(child)
}

func (r *Router) Handle(method, p string, h http.Handler) {
	full := r.pattern(method, p)
	h = chain(h, r.middlewares...)
	r.mux.Handle(full, h)
}

func (r *Router) HandleFunc(method, p string, fn http.HandlerFunc) {
	r.Handle(method, p, http.HandlerFunc(fn))
}

func (r *Router) Get(p string, h http.HandlerFunc)     { r.HandleFunc(http.MethodGet, p, h) }
func (r *Router) Post(p string, h http.HandlerFunc)    { r.HandleFunc(http.MethodPost, p, h) }
func (r *Router) Put(p string, h http.HandlerFunc)     { r.HandleFunc(http.MethodPut, p, h) }
func (r *Router) Patch(p string, h http.HandlerFunc)   { r.HandleFunc(http.MethodPatch, p, h) }
func (r *Router) Delete(p string, h http.HandlerFunc)  { r.HandleFunc(http.MethodDelete, p, h) }
func (r *Router) Head(p string, h http.HandlerFunc)    { r.HandleFunc(http.MethodHead, p, h) }
func (r *Router) Options(p string, h http.HandlerFunc) { r.HandleFunc(http.MethodOptions, p, h) }

func (r *Router) Handler(global ...Middleware) http.Handler {
	var h http.Handler = r.mux
	h = chain(h, global...)
	return h
}

func (r *Router) Timeout(d time.Duration, msg string, global ...Middleware) http.Handler {
	if msg == "" {
		msg = "request timeout"
	}
	return http.TimeoutHandler(r.Handler(global...), d, msg)
}

func (r *Router) pattern(method, p string) string {
	pp := cleanPath(joinPaths(r.base, p))
	return fmt.Sprintf("%s %s", strings.ToUpper(method), pp)
}

func joinPaths(a, b string) string {
	if a == "" {
		return cleanPath(b)
	}
	return cleanPath(path.Join(a, b))
}

func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return path.Clean(p)
}

func chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
