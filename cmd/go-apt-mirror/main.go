package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/cybozu-go/aptutil/mirror"
	"github.com/cybozu-go/log"
	"golang.org/x/net/context"
)

const (
	defaultConfigPath = "/etc/apt/mirror.toml"
)

var (
	configPath = flag.String("f", defaultConfigPath, "configuration file name")
	logLevel   = flag.String("l", "info", "log level [critical/error/warning/info/debug]")
)

func main() {
	flag.Parse()

	err := log.DefaultLogger().SetThresholdByName(*logLevel)
	if err != nil {
		log.ErrorExit(err)
	}

	var config mirror.Config
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- mirror.Run(ctx, &config, flag.Args())
	}()
	if err != nil {
		log.ErrorExit(err)
	}

	sig := make(chan os.Signal, 10)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-done:
		if err != nil {
			log.ErrorExit(err)
		}
	case <-sig:
		signal.Stop(sig)
		cancel()
		<-done
	}
}
