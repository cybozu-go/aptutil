package mirror

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/well"
	"github.com/pkg/errors"
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

	log.Info("update starts", nil)

	// run goroutines in an environment.
	env := well.NewEnvironment(ctx)

	for _, m := range ml {
		env.Go(m.Update)
	}
	env.Stop()
	err := env.Wait()

	if err != nil {
		log.Error("update failed", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	log.Info("update ends", nil)
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

	// search symlinks and its pointing directories
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

	// remove unused dentries.
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
			"path": p,
		})
		err := os.RemoveAll(p)
		if err != nil {
			return errors.Wrap(err, "gc")
		}
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
func Run(c *Config, mirrors []string) error {
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

	well.Go(func(ctx context.Context) error {
		err := updateMirrors(ctx, c, mirrors)
		if err != nil {
			if gcErr := gc(ctx, c); gcErr != nil {
				err = errors.Wrap(err, gcErr.Error())
			}
			return err
		}
		return gc(ctx, c)
	})
	well.Stop()
	return well.Wait()
}
