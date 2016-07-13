package mirror

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/cybozu-go/log"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

const (
	lockFilename = ".lock"
)

func updateMirrors(ctx context.Context, c *Config, mirrors []string) error {
	t := time.Now()

	var ml []*Mirror
	for _, id := range mirrors {
		m, err := NewMirror(t, id, c)
		if err != nil {
			return err
		}
		ml = append(ml, m)
	}

	ch := make(chan error, len(ml))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for _, m := range ml {
		go m.Update(ctx, ch)
	}
	for i := 0; i < len(ml); i++ {
		err := <-ch
		if err != nil {
			return err
		}
	}

	return nil
}

// gc removes old mirror files, if any.
func gc(ctx context.Context, c *Config) error {
	using := map[string]bool{
		lockFilename: true,
		".":          true,
		"..":         true,
	}

	dentries, err := ioutil.ReadDir(c.Dir)
	if err != nil {
		return err
	}

	for _, dentry := range dentries {
		if (dentry.Mode() & os.ModeSymlink) == 0 {
			continue
		}
		p, err := filepath.EvalSymlinks(filepath.Join(c.Dir, dentry.Name()))
		if err != nil {
			return errors.Wrap(err, "gc")
		}
		using[dentry.Name()] = true
		using[filepath.Base(filepath.Dir(p))] = true
	}

	for _, dentry := range dentries {
		if using[dentry.Name()] {
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		p := filepath.Join(c.Dir, dentry.Name())
		log.Info("removing old mirror", map[string]interface{}{
			"_path": p,
		})
		continue
		// err := os.RemoveAll(p)
		// if err != nil {
		// 	return errors.Wrap(err, "gc")
		// }
	}

	return nil
}

// Run starts mirroring.
//
// The first thing to do is to acquire flock on the lock file.
//
// mirrors is a list of mirror IDs defined in the configuration file
// (or keys in c.Mirrors).  If mirrors is an empty list, all mirrors
// will be updated.
func Run(ctx context.Context, c *Config, mirrors []string) error {
	lockFile := filepath.Join(c.Dir, lockFilename)
	f, err := os.Open(lockFile)
	switch {
	case os.IsNotExist(err):
		f2, err := os.OpenFile(lockFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return err
		}
		f = f2
	case err != nil:
		return err
	}
	defer f.Close()

	fl := Flock{f}
	err = fl.Lock()
	if err != nil {
		return err
	}
	defer fl.Unlock()

	if len(mirrors) == 0 {
		for id := range c.Mirrors {
			mirrors = append(mirrors, id)
		}
	}

	err = updateMirrors(ctx, c, mirrors)
	if err != nil {
		return err
	}
	return gc(ctx, c)
}
