//go:build unix

package texconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/e-chan1007/hayatex/internal/config"
)

func AddSystemLinks(config *config.Config) error {
	if config.InstallForAllUsers {
		err := addUnixSymlinks(config)
		if err == nil {
			return nil
		}
	}
	return addUnixProfilePath(config)
}

func addUnixProfilePath(config *config.Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	targets := []string{".profile", ".bashrc", ".zshrc"}

	line := fmt.Sprintf("\n# Added by HayaTeX\nexport PATH=\"$PATH:%s/bin/%s\"\n", config.TexDir, config.Arch)

	for _, target := range targets {
		profilePath := filepath.Join(homeDir, target)
		content, err := os.ReadFile(profilePath)
		if err != nil {
			continue
		}
		if strings.Contains(string(content), line) {
			continue
		}
		os.WriteFile(profilePath, []byte(line), 0644)
	}
	return fmt.Errorf("failed to update any profile files")

}

func addUnixSymlinks(config *config.Config) error {
	linkGroups := []struct {
		srcBase string
		dstBase string
	}{
		{filepath.Join(config.TexDir, "bin", config.Arch), config.SysBinDir},
		{filepath.Join(config.TexDir, "texmf-dist", "doc", "man"), config.SysManDir},
		{filepath.Join(config.TexDir, "texmf-dist", "doc", "info"), config.SysInfoDir},
	}

	for _, group := range linkGroups {
		if group.dstBase == "" {
			continue
		}

		if err := os.MkdirAll(group.dstBase, 0755); err != nil {
			continue // Skip if we can't create the target directory (e.g., due to permissions)
		}

		err := filepath.Walk(group.srcBase, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}

			rel, _ := filepath.Rel(group.srcBase, path)
			linkPath := filepath.Join(group.dstBase, rel)

			if err := os.MkdirAll(filepath.Dir(linkPath), 0755); err != nil {
				return err
			}

			os.Remove(linkPath)
			return os.Symlink(path, linkPath)
		})

		if err != nil {
			return err
		}
	}
	return nil
}
