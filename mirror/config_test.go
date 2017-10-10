package mirror

import (
	"reflect"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestConfig(t *testing.T) {
	t.Parallel()

	c := NewConfig()
	md, err := toml.DecodeFile("t/mirror.toml", c)
	if err != nil {
		t.Fatal(err)
	}

	if len(md.Undecoded()) > 0 {
		t.Errorf("%#v", md.Undecoded())
	}

	if c.Dir != "/var/spool/go-apt-mirror" {
		t.Error(`c.Dir != "/var/spool/go-apt-mirror"`)
	}
	if c.MaxConns != defaultMaxConns {
		t.Error(`c.MaxConns != defaultMaxConns`)
	}

	if c.Log.Level != "error" {
		t.Error(`c.Log.Level != "error"`)
	}

	if len(c.Mirrors) != 3 {
		t.Fatal(`len(c.Mirrors) != 3`)
	}

	if ubuntu, ok := c.Mirrors["ubuntu"]; !ok {
		t.Error(`ubuntu, ok := c.Mirrors["ubuntu"]; !ok`)
	} else {
		if ubuntu.URL.String() != "http://archive.ubuntu.com/ubuntu/" {
			t.Error(`ubuntu.URL != "http://archive.ubuntu.com/ubuntu/"`)
		}
		if !ubuntu.Source {
			t.Error(`!ubuntu.Source`)
		}
		if !reflect.DeepEqual(ubuntu.Architectures, []string{"amd64", "i386"}) {
			t.Error(`!reflect.DeepEqual(ubuntu.Architectures)`)
		}
		if !reflect.DeepEqual(ubuntu.Suites, []string{
			"trusty", "trusty-updates"}) {
			t.Error(`!reflect.DeepEqual(ubuntu.Suites)`)
		}
		if !reflect.DeepEqual(ubuntu.Sections, []string{
			"main", "restricted", "universe",
			"main/debian-installer",
			"restricted/debian-installer",
			"universe/debian-installer",
		}) {
			t.Error(`!reflect.DeepEqual(ubuntu.Sections)`)
		}
	}

	if security, ok := c.Mirrors["security"]; !ok {
		t.Error(`security, ok := c.Mirrors["security"]; !ok`)
	} else {
		if security.URL.String() != "http://security.ubuntu.com/ubuntu/" {
			t.Error(`security.URL != "http://security.ubuntu.com/ubuntu/"`)
		}
		if security.Source {
			t.Error(`security.Source`)
		}
		if !reflect.DeepEqual(security.Architectures, []string{"amd64"}) {
			t.Error(`!reflect.DeepEqual(security.Architectures)`)
		}
		if !reflect.DeepEqual(security.Suites, []string{"trusty-security"}) {
			t.Error(`!reflect.DeepEqual(security.Suites)`)
		}
		if !reflect.DeepEqual(security.Sections, []string{
			"main", "restricted", "universe"}) {
			t.Error(`!reflect.DeepEqual(security.Sections)`)
		}
	}
}

func TestMirrorConfig(t *testing.T) {
	t.Parallel()

	var c Config
	_, err := toml.DecodeFile("t/mirror.toml", &c)
	if err != nil {
		t.Fatal(err)
	}

	mc, ok := c.Mirrors["ubuntu"]
	if !ok {
		t.Fatal(`c.Mirrors["ubuntu"] not ok`)
	}

	correct := "http://archive.ubuntu.com/ubuntu/dists/trusty/Release"
	if mc.Resolve("dists/trusty/Release").String() != correct {
		t.Error(`mc.Resolve("dists/trusty/Release").String() != correct`)
	}

	if err := mc.Check(); err != nil {
		t.Error(err)
	}

	m := make(map[string]struct{})
	for _, p := range mc.ReleaseFiles("trusty") {
		m[p] = struct{}{}
	}
	if _, ok := m["dists/trusty/Release"]; !ok {
		t.Error(`_, ok := m["dists/trusty/Release"]; !ok`)
	}
	if _, ok := m["dists/trusty-updates/InRelease"]; ok {
		t.Error(`_, ok := m["dists/trusty-updates/InRelease"]; ok`)
	}
	for _, p := range mc.ReleaseFiles("trusty-updates") {
		m[p] = struct{}{}
	}
	if _, ok := m["dists/trusty-updates/InRelease"]; !ok {
		t.Error(`_, ok := m["dists/trusty-updates/InRelease"]; !ok`)
	}

	if !mc.MatchingIndex("hoge/fuga/Index.gz") {
		t.Error(`!mc.MatchingIndex("hoge/fuga/Index.gz")`)
	}
	if !mc.MatchingIndex("hoge/Release") {
		t.Error(`!mc.MatchingIndex("hoge/Release")`)
	}
	if mc.MatchingIndex("trusty/binary-amd64/Packages.gz") {
		t.Error(`mc.MatchingIndex("trusty/binary-amd64/Packages.gz")`)
	}
	if mc.MatchingIndex("Sources") {
		t.Error(`mc.MatchingIndex("Sources")`)
	}
	if !mc.MatchingIndex("trusty/universe/binary-all/Packages.gz") {
		t.Error(`!mc.MatchingIndex("trusty/universe/binary-all/Packages.gz")`)
	}
	if !mc.MatchingIndex("trusty/universe/binary-i386/Packages") {
		t.Error(`!mc.MatchingIndex("trusty/universe/binary-i386/Packages")`)
	}
	if !mc.MatchingIndex("trusty/main/debian-installer/source/Sources.xz") {
		t.Error(`!mc.MatchingIndex("trusty/main/debian-installer/source/Sources.xz")`)
	}

	mc, ok = c.Mirrors["security"]
	if !ok {
		t.Fatal(`c.Mirrors["security"] not ok`)
	}
	if err := mc.Check(); err != nil {
		t.Error(err)
	}
	if !mc.MatchingIndex("trusty-security/main/binary-amd64/Packages") {
		t.Error(`!mc.MatchingIndex("trusty-security/main/binary-amd64/Packages")`)
	}
	if mc.MatchingIndex("trusty-security/main/source/Sources.xz") {
		t.Error(`mc.MatchingIndex("trusty-security/main/source/Sources.xz")`)
	}

	mc, ok = c.Mirrors["flat"]
	if !ok {
		t.Fatal(`c.Mirrors["flat"] not ok`)
	}
	if err := mc.Check(); err != nil {
		t.Error(err)
	}

	m = make(map[string]struct{})
	for _, p := range mc.ReleaseFiles("12.04/") {
		m[p] = struct{}{}
	}
	if _, ok := m["12.04/Release"]; !ok {
		t.Error(`_, ok := m["12.04/Release"]; !ok`)
	}

	m = make(map[string]struct{})
	for _, p := range mc.ReleaseFiles("14.04/") {
		m[p] = struct{}{}
	}
	if _, ok := m["14.04/InRelease"]; !ok {
		t.Error(`_, ok := m["14.04/InRelease"]; !ok`)
	}

	m = make(map[string]struct{})
	for _, p := range mc.ReleaseFiles("/") {
		m[p] = struct{}{}
	}
	if _, ok := m["Release.gz"]; !ok {
		t.Error(`_, ok := m["Release.gz"]; !ok`)
	}

	correct = "http://my.local.domain/cybozu/14.04/cybozu_1.0.0_amd64.deb"
	if mc.Resolve("./14.04/cybozu_1.0.0_amd64.deb").String() != correct {
		t.Error(`mc.Resolve("14.04/cybozu_1.0.0_amd64.deb").String() != correct`)
	}
	if !mc.MatchingIndex("12.04/Packages.gz") {
		t.Error(`!mc.MatchingIndex("12.04/Packages.gz")`)
	}
	if mc.MatchingIndex("14.04/Sources") {
		t.Error(`mc.MatchingIndex("14.04/Sources")`)
	}
}
