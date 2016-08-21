package cacher

import (
	"net/http"

	"github.com/cybozu-go/cmd"
)

// NewServer returns HTTPServer implements go-apt-cacher handlers.
func NewServer(c *Cacher, config *Config) *cmd.HTTPServer {
	addr := config.Addr
	if len(addr) == 0 {
		addr = defaultAddress
	}

	return &cmd.HTTPServer{
		Server: &http.Server{
			Addr:    addr,
			Handler: cacheHandler{c},
		},
	}
}
