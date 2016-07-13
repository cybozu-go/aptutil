package mirror

import (
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/BurntSushi/toml"
)

func TestMirror(t *testing.T) {
	t.Parallel()

	c := new(Config)
	_, err := toml.DecodeFile("t/mirror.toml", c)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := NewMirror(time.Now(), "hogehoge", c); err == nil {
		t.Error(`_, err := NewMirror(time.Now(), "hogehoge", c); err == nil`)
	}

	t.Skip()

	m, err := NewMirror(time.Now(), "ubuntu", c)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	ch := make(chan error, 1)
	m.Update(ctx, ch)
	if err := <-ch; err != nil {
		t.Error(err)
	}
}
