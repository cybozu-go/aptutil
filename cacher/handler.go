package cacher

import (
	"fmt"
	"mime"
	"net"
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

	accepted := time.Now()
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
			goto LOG
		}
		stat, err := f.Stat()
		if err != nil {
			status = http.StatusInternalServerError
			http.Error(w, err.Error(), status)
			goto LOG
		}
		ct := mime.TypeByExtension(path.Ext(p))
		if ct == "" {
			ct = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
		w.WriteHeader(http.StatusOK)
	}

LOG:
	fields := map[string]interface{}{
		log.FnType:           "access",
		log.FnResponseTime:   time.Since(accepted).Seconds(),
		log.FnProtocol:       r.Proto,
		log.FnHTTPStatusCode: status,
		log.FnHTTPMethod:     r.Method,
		log.FnURL:            r.RequestURI,
		log.FnHTTPHost:       r.Host,
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		fields[log.FnRemoteAddress] = ip
	}
	ua := r.Header.Get("User-Agent")
	if len(ua) > 0 {
		fields[log.FnHTTPUserAgent] = ua
	}
	log.Info("HTTP", fields)
}
