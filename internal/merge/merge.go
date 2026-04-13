package merge

import (
	"io/ioutil"
	"os"
	"os/exec"
)

// Diff3 runs GNU diff3 -m with mine=local, older=base, yours=remote.
// Exit code 1 (overlaps) still returns merged output with conflict markers.
func Diff3(local, base, remote []byte) ([]byte, error) {
	td, err := ioutil.TempDir("", "note-diff3-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(td)
	lp := td + "/local.txt"
	bp := td + "/base.txt"
	rp := td + "/remote.txt"
	if err := ioutil.WriteFile(lp, local, 0600); err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(bp, base, 0600); err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(rp, remote, 0600); err != nil {
		return nil, err
	}
	cmd := exec.Command("diff3", "-m", lp, bp, rp)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return out, nil
		}
		return nil, err
	}
	return out, nil
}
