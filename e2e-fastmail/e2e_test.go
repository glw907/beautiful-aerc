package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var binary string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "fastmail-cli-e2e-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)

	binary = filepath.Join(tmp, "fastmail-cli")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/fastmail-cli")
	cmd.Dir = filepath.Join("..")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("build failed: " + err.Error())
	}

	os.Exit(m.Run())
}

type result struct {
	stdout string
	stderr string
	err    error
}

func run(t *testing.T, stdin string, args ...string) result {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Stdin = bytes.NewBufferString(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return result{
		stdout: stdout.String(),
		stderr: stderr.String(),
		err:    err,
	}
}

func runWithEnv(t *testing.T, env []string, stdin string, args ...string) result {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Stdin = bytes.NewBufferString(stdin)
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return result{
		stdout: stdout.String(),
		stderr: stderr.String(),
		err:    err,
	}
}
