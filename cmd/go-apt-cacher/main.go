package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/cybozu-go/aptutil/cacher"
	"github.com/cybozu-go/log"
	"golang.org/x/net/context"
)

const (
	defaultConfigPath = "/etc/go-apt-cacher.toml"
	defaultAddress    = ":3142"
	defaultMaxConns   = 10
)

var (
	configPath    = flag.String("f", defaultConfigPath, "configuration file name")
	listenAddress = flag.String("s", defaultAddress, "listen address")
	logLevel      = flag.String("l", "info", "log level [critical/error/warning/info/debug]")
)

func main() {
	flag.Parse()

	err := log.DefaultLogger().SetThresholdByName(*logLevel)
	if err != nil {
		log.ErrorExit(err)
	}

	var config cacher.Config
	md, err := toml.DecodeFile(*configPath, &config)
	if err != nil {
		log.ErrorExit(err)
	}
	if len(md.Undecoded()) > 0 {
		log.Error("invalid config keys", map[string]interface{}{
			"_keys": fmt.Sprintf("%#v", md.Undecoded()),
		})
		os.Exit(1)
	}
	if !md.IsDefined("max_conns") {
		config.MaxConns = defaultMaxConns
	}

	ctx, cancel := context.WithCancel(context.Background())
	cc, err := cacher.NewCacher(ctx, &config)
	if err != nil {
		log.ErrorExit(err)
	}

	l, err := net.Listen("tcp", *listenAddress)
	if err != nil {
		log.ErrorExit(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cacher.Serve(ctx, l, cc)
	}()

	sig := make(chan os.Signal, 10)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	signal.Stop(sig)
	cancel()
	if err := <-done; err != nil {
		log.Error(err.Error(), nil)
	}
}
