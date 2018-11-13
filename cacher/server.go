package cacher

import (
	"net/http"

	"github.com/cybozu-go/well"
)

// NewServer returns HTTPServer implements go-apt-cacher handlers.
func NewServer(c *Cacher, config *Config) *well.HTTPServer {
	addr := config.Addr
	if len(addr) == 0 {
		addr = defaultAddress
	}

	return &well.HTTPServer{
		Server: &http.Server{
			Addr:    addr,
			Handler: cacheHandler{c},
		},
	}
}
