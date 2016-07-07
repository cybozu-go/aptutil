package cacher

import (
	"net/url"
	"testing"
)

func TestURLMap(t *testing.T) {
	t.Parallel()

	um := make(URLMap)
	u, _ := url.Parse("http://archive.ubuntu.com/ubuntu")

	err := um.Register("", u)
	if err != ErrInvalidPrefix {
		t.Error(`empty prefix must be invalid`)
	}

	err = um.Register("hoge/fuga", u)
	if err != ErrInvalidPrefix {
		t.Error(`hoge/fuga must be an invalid prefix`)
	}

	err = um.Register("ubuntu", u)
	if err != nil {
		t.Error(`ubuntu must be a valid prefix`)
	}

	if um.URL("hoge/fuga") != nil {
		t.Error(`um.URL("hoge/fuga") != nil`)
	}

	if u2 := um.URL("ubuntu"); u2 == nil {
		t.Error(`u2 := um.URL("ubuntu"); u2 == nil`)
	} else {
		if u2.String() != "http://archive.ubuntu.com/ubuntu/" {
			t.Error(`u2.String() != "http://archive.ubuntu.com/ubuntu/"`)
		}
	}

	if u2 := um.URL("ubuntu/"); u2 == nil {
		t.Error(`u2 := um.URL("ubuntu/"); u2 == nil`)
	} else {
		if u2.String() != "http://archive.ubuntu.com/ubuntu/" {
			t.Error(`u2.String() != "http://archive.ubuntu.com/ubuntu/"`)
		}
	}

	if u2 := um.URL("ubuntu/dists/trusty/Release"); u2 == nil {
		t.Error(`u2 := um.URL("ubuntu/dists/trusty/Release"); u2 == nil`)
	} else {
		if u2.String() != "http://archive.ubuntu.com/ubuntu/dists/trusty/Release" {
			t.Error(`u2.String() != "http://archive.ubuntu.com/ubuntu/dists/trusty/Release"`)
			t.Log(u2.String())
		}
	}
}
