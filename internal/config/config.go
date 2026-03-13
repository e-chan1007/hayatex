package config

type Config struct {
	MirrorURL          string
	TexDir             string
	Arch               string
	AddPath            bool
	InstallDocFiles    bool
	InstallSrcFiles    bool
	InstallForAllUsers bool
	SysBinDir          string
	SysManDir          string
	SysInfoDir         string
	RootPackages       []string
}
