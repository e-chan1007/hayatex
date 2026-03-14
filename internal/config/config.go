package config

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Config struct {
	MirrorURL          string
	TexDir             string
	TexmfLocalDir      string
	Arch               string
	AddPath            bool
	InstallDocFiles    bool
	InstallSrcFiles    bool
	InstallForAllUsers bool
	SysBinDir          string
	SysManDir          string
	SysInfoDir         string
	RootPackages       []string
	IsPortable         bool
}

func NewDefaultConfig() *Config {
	return &Config{
		MirrorURL:          "https://mirror.ctan.org/systems/texlive/tlnet/",
		TexDir:             "",
		TexmfLocalDir:      "",
		Arch:               "",
		AddPath:            false,
		InstallDocFiles:    false,
		InstallSrcFiles:    false,
		InstallForAllUsers: false,
		SysBinDir:          "/usr/local/bin",
		SysManDir:          "/usr/local/share/man",
		SysInfoDir:         "/usr/local/share/info",
	}
}

func (c *Config) SetDefaultTeXDir(year string) {
	c.TexDir = filepath.Join("/usr/local/texlive", year)
	if runtime.GOOS == "windows" {
		c.TexDir = filepath.Join("C:\\texlive", year)
	}
	c.TexmfLocalDir = filepath.Join(c.TexDir, "../texmf-local")
}

func LoadProfile(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := NewDefaultConfig()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch {
		case key == "TEXDIR":
			config.TexDir = val
		case key == "TEXMFLOCAL":
			config.TexmfLocalDir = val
		case key == "selected_scheme":
			if val != "scheme-custom" {
				config.RootPackages = append(config.RootPackages, val)
			}
		case key == "option_doc":
			config.InstallDocFiles = (val == "1")
		case key == "option_src":
			config.InstallSrcFiles = (val == "1")
		case key == "option_path":
			config.AddPath = (val == "1")
		case key == "option_sys_bin":
			config.SysBinDir = val
		case key == "option_sys_man":
			config.SysManDir = val
		case key == "option_sys_info":
			config.SysInfoDir = val
		case key == "tlpobj_repository":
			config.MirrorURL = val
		case key == "tlpdbopt_install_docfiles":
			config.InstallDocFiles = (val == "1")
		case key == "tlpdbopt_install_srcfiles":
			config.InstallSrcFiles = (val == "1")
		case key == "instopt_portable":
			config.AddPath = false
		case strings.HasPrefix(key, "collection-"):
			if val == "1" {
				config.RootPackages = append(config.RootPackages, key)
			}
		case strings.HasPrefix(key, "binary-"):
			if val == "1" {
				config.Arch = strings.TrimPrefix(key, "binary-")
			}
		}
	}
	return config, scanner.Err()
}
