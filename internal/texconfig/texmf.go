package texconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/e-chan1007/hayatex/internal/config"
)

func GenerateTexmfConfig(cfg *config.Config) error {
	content := fmt.Sprintf(`
TEXMFDIST = $SELFAUTOPARENT/texmf-dist
TEXMFSYSVAR = $SELFAUTOPARENT/texmf-var
TEXMFSYSCONFIG = $SELFAUTOPARENT/texmf-config
TEXMFLOCAL = %s
`, cfg.TexmfLocalDir) // プロファイルから計算

	configPath := filepath.Join(cfg.TexDir, "texmf.cnf")
	return os.WriteFile(configPath, []byte(strings.TrimSpace(content)), 0644)
}
