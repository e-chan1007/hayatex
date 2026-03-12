package utils

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/e-chan1007/hayatex/internal/config"
)

type CommandExecutor struct {
	binDir string
	dir    string
	env    []string
	stdout *io.Writer
	stderr *io.Writer
}

func TeXCommandExecutor(config *config.Config, out io.Writer) CommandExecutor {
	binDir := filepath.Join(config.TexDir, "bin", config.Arch)
	newPath := binDir + string(os.PathListSeparator) + os.Getenv("PATH")
	if config.Arch == "windows" {
		perlDir := filepath.Join(config.TexDir, "tlpkg", "tlperl", "bin")
		newPath = perlDir + string(os.PathListSeparator) + newPath
	}
	env := os.Environ()
	env = SetEnv(env, "PATH", newPath)
	env = SetEnv(env, "TEXMFROOT", config.TexDir)
	env = SetEnv(env, "PERL5LIB", filepath.Join(config.TexDir, "tlpkg"))

	return CommandExecutor{
		binDir: binDir,
		dir:    config.TexDir,
		env:    env,
		stdout: &out,
		stderr: &out,
	}
}

func (ce CommandExecutor) NewCommand(name string, args ...string) *exec.Cmd {
	cmdPath, err := ce.resolveCommand(name)
	if err != nil {
		log.Fatalf("Failed to resolve executable for %s: %v", name, err)
	}
	cmd := exec.Command(cmdPath, args...)
	ce.injectOptions(cmd)
	return cmd
}

func (ce CommandExecutor) NewCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmdPath, err := ce.resolveCommand(name)
	if err != nil {
		log.Fatalf("Failed to resolve executable for %s: %v", name, err)
	}
	cmd := exec.CommandContext(ctx, cmdPath, args...)
	ce.injectOptions(cmd)
	return cmd
}

func (ce CommandExecutor) resolveCommand(name string) (string, error) {
	if ce.binDir == "" {
		return name, nil
	}
	return ResolveExecutable(ce.binDir, name)
}

func (ce CommandExecutor) injectOptions(cmd *exec.Cmd) {
	cmd.Dir = ce.dir
	cmd.Env = ce.env
	cmd.Stdout = *ce.stdout
	cmd.Stderr = *ce.stderr
}

var resolvedExecutables = make(map[string]string)

func ResolveExecutable(paths ...string) (string, error) {
	path := filepath.Join(paths...)
	if runtime.GOOS != "windows" {
		return path, nil
	}

	if resolved, ok := resolvedExecutables[path]; ok {
		return resolved, nil
	}

	pathExt := os.Getenv("PATHEXT")
	extensions := strings.Split(strings.ToUpper(pathExt), ";")
	candidates := append([]string{""}, extensions...)

	for _, ext := range candidates {
		fullPath := filepath.Join(filepath.Dir(path), filepath.Base(path)+ext)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			resolvedExecutables[path] = fullPath
			return fullPath, nil
		}
	}

	return "", fmt.Errorf("executable not found: %s", path)
}
