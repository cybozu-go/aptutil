package aptcacher

import (
	_log "log"
	"net"
	"net/http"
	"time"

	"github.com/cybozu-go/log"
	"github.com/facebookgo/httpdown"
	"golang.org/x/net/context"
)

const (
	defaultReadTimeout  = 10 * time.Second
	defaultWriteTimeout = 10 * time.Second
)

// Serve runs REST API server until ctx.Done() is closed.
func Serve(ctx context.Context, l net.Listener, c *Cacher) error {
	hd := httpdown.HTTP{}
	logger := _log.New(log.DefaultLogger().Writer(log.LvError), "[http]", 0)
	s := &http.Server{
		Handler:      cacheHandler{c},
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
		ErrorLog:     logger,
	}
	hs := hd.Serve(s, l)

	waiterr := make(chan error, 1)
	go func() {
		defer close(waiterr)
		waiterr <- hs.Wait()
	}()

	select {
	case err := <-waiterr:
		return err

	case <-ctx.Done():
		if err := hs.Stop(); err != nil {
			return err
		}
		return <-waiterr
	}
}
