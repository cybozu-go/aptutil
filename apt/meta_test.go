package apt

import (
	"encoding/hex"
	"os"
	"testing"
)

func TestIsMeta(t *testing.T) {
	if IsMeta("hoge.deb") {
		t.Error(`IsMeta("hoge.deb")`)
	}
	if IsMeta("Release/hoge") {
		t.Error(`IsMeta("Release/hoge")`)
	}
	if !IsMeta("Release") {
		t.Error(`!IsMeta("Release")`)
	}
	if !IsMeta("Release.gpg") {
		t.Error(`!IsMeta("Release.gpg")`)
	}
	if !IsMeta("InRelease") {
		t.Error(`!IsMeta("InRelease")`)
	}
	if !IsMeta("Packages") {
		t.Error(`!IsMeta("Packages")`)
	}
	if !IsMeta("Packages.gz") {
		t.Error(`!IsMeta("Packages.gz")`)
	}
	if !IsMeta("Packages.bz2") {
		t.Error(`!IsMeta("Packages.bz2")`)
	}
	if !IsMeta("Packages.xz") {
		t.Error(`!IsMeta("Packages.xz")`)
	}
	if IsMeta("Packages.gz.xz") {
		t.Error(`IsMeta("Packages.gz.xz")`)
	}
	if !IsMeta("a/b/c/Sources.gz") {
		t.Error(`!IsMeta("a/b/c/Sources.gz")`)
	}
	if !IsMeta("Index") {
		t.Error(`!IsMeta("Index")`)
	}
}

func containsFileInfo(fi *FileInfo, l []*FileInfo) bool {
	for _, fi2 := range l {
		if fi.Same(fi2) {
			return true
		}
	}
	return false
}

func TestAcquireByHash(t *testing.T) {
	t.Parallel()

	f, err := os.Open("testdata/hash/Release")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	_, d, err := ExtractFileInfo("ubuntu/dists/trusty/Release", f)
	if err != nil {
		t.Fatal(err)
	}
	if !SupportByHash(d) {
		t.Error(`!SupportByHash(d)`)
	}
}

func TestGetFilesFromRelease(t *testing.T) {
	t.Parallel()

	f, err := os.Open("testdata/af/Release")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	fil, d, err := ExtractFileInfo("ubuntu/dists/trusty/Release", f)
	if err != nil {
		t.Fatal(err)
	}
	if len(fil) != 9 {
		t.Error(`len(fil) != 9`)
	}

	if SupportByHash(d) {
		t.Error(`SupportByHash(d)`)
	}

	md5sum, _ := hex.DecodeString("5c30f072d01cde094a5c07fccd217cf3")
	sha1sum, _ := hex.DecodeString("e3c9a2028a6938e49fc240cdd55c2f4b0b75dfde")
	sha256sum, _ := hex.DecodeString("e3b1e5a6951881bca3ee230e5f3215534eb07f602a2f0415af3b182468468104")
	fi := &FileInfo{
		path:      "ubuntu/dists/trusty/main/binary-all/Packages",
		size:      3098,
		md5sum:    md5sum,
		sha1sum:   sha1sum,
		sha256sum: sha256sum,
	}
	if !containsFileInfo(fi, fil) {
		t.Error(`ubuntu/dists/trusty/main/binary-all/Packages`)
	}

	md5sum, _ = hex.DecodeString("3f71c3b19ec6f926c71504cf147f3574")
	sha1sum, _ = hex.DecodeString("64a566a5b6a92c1fefde9630d1b8ecb6e9352523")
	sha256sum, _ = hex.DecodeString("78fa82404a432d7b56761ccdbf275f4a338c8779a9cec17480b91672c28682aa")
	fi = &FileInfo{
		path:      "ubuntu/dists/trusty/main/binary-amd64/Packages.gz",
		size:      4418,
		md5sum:    md5sum,
		sha1sum:   sha1sum,
		sha256sum: sha256sum,
	}
	if !containsFileInfo(fi, fil) {
		t.Error(`ubuntu/dists/trusty/main/binary-amd64/Packages.gz`)
	}
}

func TestGetFilesFromPackages(t *testing.T) {
	t.Parallel()

	f, err := os.Open("testdata/af/Packages")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	fil, _, err := ExtractFileInfo("ubuntu/dists/testing/main/binary-amd64/Packages", f)
	if err != nil {
		t.Fatal(err)
	}
	if len(fil) != 3 {
		t.Error(`len(fil) != 3`)
	}

	sha1sum, _ := hex.DecodeString("903b3305c86e872db25985f2b686ef8d1c3760cf")
	sha256sum, _ := hex.DecodeString("cebb641f03510c2c350ea2e94406c4c09708364fa296730e64ecdb1107b380b7")
	fi := &FileInfo{
		path:      "pool/c/cybozu-abc_0.2.2-1_amd64.deb",
		size:      102369852,
		sha1sum:   sha1sum,
		sha256sum: sha256sum,
	}
	if !fi.Same(fil[0]) {
		t.Error(`!fi.Same(fil[0])`)
	}

	sha1sum, _ = hex.DecodeString("b89e2f1a9f5efb8b7c1e2e2d8abbab05d7981187")
	sha256sum, _ = hex.DecodeString("814cec015067fb083e14d95d77c5ec41c11de99180ea518813b7abc88805fa24")
	fi = &FileInfo{
		path:      "pool/c/cybozu-fuga_2.0.0.2-1_all.deb",
		size:      1018650,
		sha1sum:   sha1sum,
		sha256sum: sha256sum,
	}
	if !fi.Same(fil[1]) {
		t.Error(`!fi.Same(fil[1])`)
	}
}

func TestGetFilesFromSources(t *testing.T) {
	t.Parallel()

	f, err := os.Open("testdata/af/Sources.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	fil, _, err := ExtractFileInfo("ubuntu/dists/testing/main/source/Sources.gz", f)
	if err != nil {
		t.Fatal(err)
	}
	if len(fil) < 2 {
		t.Error(`len(fil) < 2`)
	}

	md5sum, _ := hex.DecodeString("6cfe5a56e3b0fc25edf653084c24c238")
	sha1sum, _ := hex.DecodeString("d89f409cae51a5d424a769560fc1688d2a636d73")
	sha256sum, _ := hex.DecodeString("3a126eec194457778a477d95a9dd4b8c03d6a95b9c064cddcae63eba2e674797")
	fi := &FileInfo{
		path:      "pool/main/a/aalib/aalib_1.4p5-41.dsc",
		size:      2078,
		md5sum:    md5sum,
		sha1sum:   sha1sum,
		sha256sum: sha256sum,
	}
	if !containsFileInfo(fi, fil) {
		t.Error(`pool/main/a/aalib/aalib_1.4p5-41.dsc`)
	}

	md5sum, _ = hex.DecodeString("9801095c42bba12edebd1902bcf0a990")
	sha1sum, _ = hex.DecodeString("a23269e950a249d2ef93625837cace45ddbce03b")
	sha256sum, _ = hex.DecodeString("fbddda9230cf6ee2a4f5706b4b11e2190ae45f5eda1f0409dc4f99b35e0a70ee")
	fi = &FileInfo{
		path:      "pool/main/a/aalib/aalib_1.4p5.orig.tar.gz",
		size:      391028,
		md5sum:    md5sum,
		sha1sum:   sha1sum,
		sha256sum: sha256sum,
	}
	if !containsFileInfo(fi, fil) {
		t.Error(`pool/main/a/aalib/aalib_1.4p5.orig.tar.gz`)
	}

	md5sum, _ = hex.DecodeString("1d276558e27a29e2d0bbe6deac1788dc")
	sha1sum, _ = hex.DecodeString("bfe56ce2a2171c6602f4d34a4d548a20deb2e628")
	sha256sum, _ = hex.DecodeString("0b606e2bf1826e77c73c0efb9b0cb2f5f89ea422cc02a10fa00866075635cf2c")
	fi = &FileInfo{
		path:      "pool/main/a/aalib/aalib_1.4p5-41.debian.tar.gz",
		size:      16718,
		md5sum:    md5sum,
		sha1sum:   sha1sum,
		sha256sum: sha256sum,
	}
	if !containsFileInfo(fi, fil) {
		t.Error(`pool/main/a/aalib/aalib_1.4p5-41.debian.tar.gz`)
	}

	md5sum, _ = hex.DecodeString("7dedd7a510fcf4cd2b0def4b45ab94a7")
	sha1sum, _ = hex.DecodeString("fcaf0374f5f054c2884dbab6f126b8187ba66181")
	sha256sum, _ = hex.DecodeString("8f44b8be08a562ac7bee3bd5e0273e6a860bfe1a434ea2d93d42e94d339cacf4")
	fi = &FileInfo{
		path:      "pool/main/z/zsh/zsh_5.0.2-3ubuntu6.dsc",
		size:      2911,
		md5sum:    md5sum,
		sha1sum:   sha1sum,
		sha256sum: sha256sum,
	}
	if !containsFileInfo(fi, fil) {
		t.Error(`pool/main/z/zsh/zsh_5.0.2-3ubuntu6.dsc`)
	}
}

func TestGetFilesFromIndex(t *testing.T) {
	t.Parallel()

	f, err := os.Open("testdata/af/Index")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	fil, _, err := ExtractFileInfo("ubuntu/dists/trusty/main/i18n/Index", f)
	if err != nil {
		t.Fatal(err)
	}
	if len(fil) != 53 {
		t.Error(`len(fil) != 53`)
	}

	sha1sum, _ := hex.DecodeString("f03d5f043a7daea0662a110d6e5d3f85783a5a1b")
	fi := &FileInfo{
		path:    "ubuntu/dists/trusty/main/i18n/Translation-bg.bz2",
		size:    7257,
		sha1sum: sha1sum,
	}
	if !containsFileInfo(fi, fil) {
		t.Error(`ubuntu/dists/trusty/main/i18n/Translation-bg.bz2`)
	}

	sha1sum, _ = hex.DecodeString("1572e835b4a67a49f79bbee408c82af2357662a7")
	fi = &FileInfo{
		path:    "ubuntu/dists/trusty/main/i18n/Translation-zh_TW.bz2",
		size:    85235,
		sha1sum: sha1sum,
	}
	if !containsFileInfo(fi, fil) {
		t.Error(`ubuntu/dists/trusty/main/i18n/Translation-zh_TW.bz2`)
	}
}

func TestExtractFileInfo(t *testing.T) {
	t.Parallel()

	f, err := os.Open("testdata/af/Packages")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	fil, _, err := ExtractFileInfo("ubuntu/dists/testing/Release.gpg", f)
	if err != nil {
		t.Fatal(err)
	}
	if len(fil) != 0 {
		t.Error(`len(fil) != 0`)
	}
}

func TestExtractFileInfoWithXZ(t *testing.T) {
	t.Parallel()

	f, err := os.Open("testdata/af/Packages.xz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	fil, _, err := ExtractFileInfo("ubuntu/dists/testing/Packages.xz", f)
	if err != nil {
		t.Fatal(err)
	}

	sha1sum, _ := hex.DecodeString("903b3305c86e872db25985f2b686ef8d1c3760cf")
	fi := &FileInfo{
		path:    "pool/c/cybozu-abc_0.2.2-1_amd64.deb",
		size:    102369852,
		sha1sum: sha1sum,
	}
	if !containsFileInfo(fi, fil) {
		t.Error("pool/c/cybozu-abc_0.2.2-1_amd64.deb")
	}
}
