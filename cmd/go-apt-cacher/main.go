package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/cybozu-go/aptutil/cacher"
	"github.com/cybozu-go/cmd"
	"github.com/cybozu-go/log"
)

const (
	defaultConfigPath = "/etc/go-apt-cacher.toml"
)

var (
	configPath = flag.String("f", defaultConfigPath, "configuration file name")
)

func main() {
	flag.Parse()

	config := cacher.NewConfig()
	md, err := toml.DecodeFile(*configPath, &config)
	if err != nil {
		log.ErrorExit(err)
	}
	if len(md.Undecoded()) > 0 {
		log.Error("invalid config keys", map[string]interface{}{
			"keys": fmt.Sprintf("%#v", md.Undecoded()),
		})
		os.Exit(1)
	}

	err = config.Log.Apply()
	if err != nil {
		log.ErrorExit(err)
	}
	cc, err := cacher.NewCacher(config)
	if err != nil {
		log.ErrorExit(err)
	}

	s := cacher.NewServer(cc, config)
	err = s.ListenAndServe()
	if err != nil {
		log.ErrorExit(err)
	}

	err = cmd.Wait()
	if err != nil && !cmd.IsSignaled(err) {
		log.ErrorExit(err)
	}
}
