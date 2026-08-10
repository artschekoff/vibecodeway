package github

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// stubRunner returns a Runner whose gh binary is a shell stub that appends its
// args (one per line, space-joined) to argsFile and exits 0.
func stubRunner(t *testing.T) (*Runner, string) {
	t.Helper()
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	stub := filepath.Join(dir, "gh")
	script := "#!/bin/sh\necho \"$@\" >> " + argsFile + "\n"
	require.NoError(t, os.WriteFile(stub, []byte(script), 0o755))
	return &Runner{binPath: stub, timeout: 5 * time.Second, logger: zap.NewNop()}, argsFile
}

func TestUpdatePRBranch_InvokesGH(t *testing.T) {
	r, argsFile := stubRunner(t)
	require.NoError(t, r.UpdatePRBranch(context.Background(), "acme", "web", 7))

	out, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	require.Equal(t, "pr update-branch 7 --repo acme/web", strings.TrimSpace(string(out)))
}
