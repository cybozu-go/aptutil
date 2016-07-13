package mirror

import "testing"

func TestDirSync(t *testing.T) {
	t.Parallel()

	err := DirSync(".")
	if err != nil {
		t.Error(err)
	}

	err = DirSyncTree(".")
	if err != nil {
		t.Error(err)
	}
}
