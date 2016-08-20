package cacher

import (
	"fmt"
	"mime"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/cybozu-go/log"
)

type cacheHandler struct {
	*Cacher
}

func (c cacheHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET", "HEAD":
		// later on
	default:
		http.Error(w, "bad method", http.StatusNotImplemented)
		return
	}

	p := path.Clean(r.URL.Path[1:])

	if log.Enabled(log.LvDebug) {
		log.Debug("request path", map[string]interface{}{
			"path": p,
		})
	}

	status, f, err := c.Get(p)

	switch {
	case err != nil:
		http.Error(w, err.Error(), status)
	case status == http.StatusNotFound:
		http.NotFound(w, r)
	case status != http.StatusOK:
		http.Error(w, fmt.Sprintf("status %d", status), status)
	default:
		// http.StatusOK
		defer f.Close()
		if r.Method == "GET" {
			var zeroTime time.Time
			http.ServeContent(w, r, path.Base(p), zeroTime, f)
			return
		}
		stat, err := f.Stat()
		if err != nil {
			status = http.StatusInternalServerError
			http.Error(w, err.Error(), status)
			return
		}
		ct := mime.TypeByExtension(path.Ext(p))
		if ct == "" {
			ct = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
		w.WriteHeader(http.StatusOK)
	}
}
