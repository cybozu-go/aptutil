package apt

import (
	"io"
	"os"
	"testing"
)

func TestParserRelease(t *testing.T) {
	t.Parallel()

	f, err := os.Open("testdata/af/Release")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	p := NewParser(f)
	d, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if codename, ok := d["Codename"]; !ok {
		t.Error(`codename, ok := d["Codename"]; !ok`)
	} else if codename[0] != "testing" {
		t.Error(`codename != "testing"`)
	}
	if archs, ok := d["Architectures"]; !ok {
		t.Error(`archs, ok := d["Architectures"]; !ok`)
	} else if archs[0] != "amd64 i386" {
		t.Error(`archs[0] != "amd64 i386"`)
	}

	if md5, ok := d["MD5Sum"]; !ok {
		t.Error(`md5, ok := d["MD5Sum"]; !ok`)
	} else {
		if len(md5) != 9 {
			t.Fatal(`len(md5) != 9`)
		}
		if md5[0] != "5c30f072d01cde094a5c07fccd217cf3             3098 main/binary-all/Packages" {
			t.Error(`md5[0] != "5c30f072d01cde094a5c07fccd217cf3             3098 main/binary-all/Packages"`)
		}
		if md5[1] != "4ed86bda6871fd3825a65e95bb714ef0             1259 main/binary-all/Packages.bz2" {
			t.Error(`md5[1] != "4ed86bda6871fd3825a65e95bb714ef0             1259 main/binary-all/Packages.bz2"`)
		}
	}

	if sha1, ok := d["SHA1"]; !ok {
		t.Error(`sha1, ok := d["SHA1"]; !ok`)
	} else {
		if len(sha1) != 9 {
			t.Fatal(`len(sha1) != 9`)
		}
		if sha1[0] != "e3c9a2028a6938e49fc240cdd55c2f4b0b75dfde             3098 main/binary-all/Packages" {
			t.Error(`sha1[0] != "e3c9a2028a6938e49fc240cdd55c2f4b0b75dfde             3098 main/binary-all/Packages"`)
		}
		if sha1[1] != "eb2c25b19facbc8c103a7e14ae5b768e5e47157e             1259 main/binary-all/Packages.bz2" {
			t.Error(`sha1[1] != "eb2c25b19facbc8c103a7e14ae5b768e5e47157e             1259 main/binary-all/Packages.bz2"`)
		}
	}

	if sha256, ok := d["SHA256"]; !ok {
		t.Error(`sha256, ok := d["SHA256"]; !ok`)
	} else {
		if len(sha256) != 9 {
			t.Fatal(`len(sha256) != 9`)
		}
		if sha256[0] != "e3b1e5a6951881bca3ee230e5f3215534eb07f602a2f0415af3b182468468104             3098 main/binary-all/Packages" {
			t.Error(`sha256[0] != "e3b1e5a6951881bca3ee230e5f3215534eb07f602a2f0415af3b182468468104             3098 main/binary-all/Packages"`)
		}
		if sha256[8] != "a6972328347cc787f4f8c2e20a930ec965bd520380b0449e610995b6b0f1e3c5             1059 main/binary-i386/Packages.gz" {
			t.Error(`sha256[8] != "a6972328347cc787f4f8c2e20a930ec965bd520380b0449e610995b6b0f1e3c5             1059 main/binary-i386/Packages.gz"`)
		}
	}

	_, err = p.Read()
	if err != io.EOF {
		t.Error(`err != io.EOF`)
	}
}

func TestParserInRelease(t *testing.T) {
	t.Parallel()

	f, err := os.Open("testdata/af/InRelease")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	p := NewParser(f)
	d, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if codename, ok := d["Codename"]; !ok {
		t.Error(`codename, ok := d["Codename"]; !ok`)
	} else if codename[0] != "xenial" {
		t.Error(`codename != "xenial"`)
	}
	if components, ok := d["Components"]; !ok {
		t.Error(`components, ok := d["Components"]; !ok`)
	} else if components[0] != "main restricted universe multiverse" {
		t.Error(`components[0] != "main restricted universe multiverse"`)
	}

	if sha256, ok := d["SHA256"]; !ok {
		t.Error(`sha256, ok := d["SHA256"]; !ok`)
	} else {
		if sha256[len(sha256)-1] != "aefe5a7388a3e638df10ac8f0cd42e6c2947cc766c2f33a3944a5b4900369d1e          7727612 universe/source/Sources.xz" {
			t.Error(`sha256[len(sha256)-1] != "aefe5a7388a3e638df10ac8f0cd42e6c2947cc766c2f33a3944a5b4900369d1e          7727612 universe/source/Sources.xz"`)
		}
	}

	_, err = p.Read()
	if err != io.EOF {
		t.Error(`err != io.EOF`)
	}
}

func TestParserPackages(t *testing.T) {
	t.Parallel()

	f, err := os.Open("testdata/af/Packages")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	p := NewParser(f)
	d, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if pkg, ok := d["Package"]; !ok {
		t.Error(`pkg, ok := d["Package"]; !ok`)
	} else if pkg[0] != "cybozu-abc" {
		t.Error(`pkg[0] != "cybozu-abc"`)
	}
	if filename, ok := d["Filename"]; !ok {
		t.Error(`filename, ok := d["Filename"]; !ok`)
	} else if filename[0] != "pool/c/cybozu-abc_0.2.2-1_amd64.deb" {
		t.Error(`filename[0] != "pool/c/cybozu-abc_0.2.2-1_amd64.deb"`)
	}
	if size, ok := d["Size"]; !ok {
		t.Error(`size, ok := d["Size"]; !ok`)
	} else if size[0] != "102369852" {
		t.Error(`size[0] != "102369852"`)
	}

	d, err = p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if pkg, ok := d["Package"]; !ok {
		t.Error(`pkg, ok := d["Package"]; !ok`)
	} else if pkg[0] != "cybozu-fuga" {
		t.Error(`pkg[0] != "cybozu-fuga"`)
	}
	if filename, ok := d["Filename"]; !ok {
		t.Error(`filename, ok := d["Filename"]; !ok`)
	} else if filename[0] != "pool/c/cybozu-fuga_2.0.0.2-1_all.deb" {
		t.Error(`filename[0] != "pool/c/cybozu-fuga_2.0.0.2-1_all.deb"`)
	}
	if size, ok := d["Size"]; !ok {
		t.Error(`size, ok := d["Size"]; !ok`)
	} else if size[0] != "1018650" {
		t.Error(`size[0] != "1018650"`)
	}

	_, err = p.Read()
	if err != io.EOF {
		t.Error(`err != io.EOF`)
	}
}
