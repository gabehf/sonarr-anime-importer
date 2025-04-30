package main

import (
	"bytes"
	"log"
	"net/http"
	"time"

	"github.com/patrickmn/go-cache"
)

// from https://medium.com/@chrisgregory_83433/chaining-middleware-in-go-918cfbc5644d
type Middleware func(http.HandlerFunc) http.HandlerFunc

func ChainMiddleware(h http.HandlerFunc, m ...Middleware) http.HandlerFunc {
	if len(m) < 1 {
		return h
	}
	wrapped := h
	// loop in reverse to preserve middleware order
	for i := len(m) - 1; i >= 0; i-- {
		wrapped = m[i](wrapped)
	}
	return wrapped
}

func newRebuildStaleIdMapMiddleware(idMap *ConcurrentMap) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if time.Since(lastBuiltAnimeIdList) > 24*time.Hour {
				log.Println("Anime ID association table expired, building new table...")
				buildIdMap(idMap)
			}
			next.ServeHTTP(w, r)
		})
	}
}

type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func loggerMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		srw := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK} // default to 200
		next.ServeHTTP(srw, r)

		duration := time.Since(start)
		log.Printf("%s - %d %s - %v", RequestString(r), srw.status, http.StatusText(srw.status), duration)
	})
}

type cacheResponseWriter struct {
	http.ResponseWriter
	status int
	body   *bytes.Buffer
}

func (w *cacheResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *cacheResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b) // Capture body
	return w.ResponseWriter.Write(b)
}

func newCacheMiddleware(c *cache.Cache) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := RequestString(r)
			if cachedResp, found := c.Get(key); found {
				log.Println("Responding with cached response")
				w.WriteHeader(http.StatusOK)
				w.Write(cachedResp.([]byte))
				return
			}
			crw := &cacheResponseWriter{
				ResponseWriter: w,
				body:           &bytes.Buffer{},
			}
			next.ServeHTTP(crw, r)
			if crw.status == http.StatusOK {
				c.Set(key, crw.body.Bytes(), cache.DefaultExpiration)
			}
		})
	}
}
