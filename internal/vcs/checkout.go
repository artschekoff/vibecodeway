package vcs

import (
	"fmt"
	"os/exec"
)

// GitCheckout switches the working tree at dir to the given branch.
func GitCheckout(dir, branch string) error {
	cmd := exec.Command("git", "-C", dir, "checkout", branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout %s in %s: %w\n%s", branch, dir, err, out)
	}
	return nil
}
