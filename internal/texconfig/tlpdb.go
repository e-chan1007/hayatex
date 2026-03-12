package texconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/e-chan1007/hayatex/internal/config"
	"github.com/e-chan1007/hayatex/internal/resolver"
	"github.com/e-chan1007/hayatex/internal/utils"
)

func SaveLocalTLPDB(config *config.Config, tlpdb *resolver.TLDatabase, deps *resolver.TLDatabase) error {
	saveDeps := make(resolver.TLDatabase)
	for name := range *deps {
		saveDeps[name] = (*tlpdb)[name]
	}
	saveDeps["00texlive.config"] = (*tlpdb)["00texlive.config"]
	saveDeps["00texlive.installation"] = createTeXLiveInstallationConfig(config)

	f, err := os.Create(filepath.Join(config.TexDir, "tlpkg/texlive.tlpdb"))
	if err != nil {
		return err
	}
	defer f.Close()
	f.WriteString(saveDeps.ToString(config))
	return nil
}

func createTeXLiveInstallationConfig(config *config.Config) *resolver.TLPackage {
	depends := []string{
		fmt.Sprintf("opt_location:%s", config.MirrorURL),
		fmt.Sprintf("opt_install_docfiles:%d", utils.BoolToInt(config.InstallDocFiles)),
		fmt.Sprintf("opt_install_srcfiles:%d", utils.BoolToInt(config.InstallSrcFiles)),
		"opt_create_formats:1",
		fmt.Sprintf("setting_available_architectures:%s", config.Arch),
	}

	if config.Arch == "windows" {
		depends = append(depends, fmt.Sprintf("opt_w32_multi_user:%d", utils.BoolToInt(config.InstallForAllUsers)))
	}

	if strings.Contains(config.Arch, "linux") || strings.Contains(config.Arch, "darwin") {
		depends = append(depends, fmt.Sprintf("setting_sys_bin:%s", config.SysBinDir))
		depends = append(depends, fmt.Sprintf("setting_sys_man:%s", config.SysManDir))
		depends = append(depends, fmt.Sprintf("setting_sys_info:%s", config.SysInfoDir))
	}

	return &resolver.TLPackage{
		Name: "00texlive.installation",
		Container: &resolver.TLContainerInfo{
			Size:     0,
			Checksum: "",
		},
		Depends: depends,
	}
}
