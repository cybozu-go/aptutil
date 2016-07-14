package mirror

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestFlock(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("flock", "t/mirror.toml", "sleep", "0.2")
	err := cmd.Start()
	if err != nil {
		t.Skip()
		return
	}
	time.Sleep(100 * time.Millisecond)

	f, err := os.Open("t/mirror.toml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	fl := Flock{f}
	if err = fl.Lock(); err == nil {
		t.Error(`err = fl.Lock(); err == nil`)
	} else {
		t.Log(err)
	}

	cmd.Wait()
	if err = fl.Lock(); err != nil {
		t.Fatal(err)
	}
	if err = fl.Unlock(); err != nil {
		t.Error(err)
	}
}
