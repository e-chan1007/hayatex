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
TEXMFLOCAL = %s
`, cfg.TexmfLocalDir)

	configPath := filepath.Join(cfg.TexDir, "texmf.cnf")
	return os.WriteFile(configPath, []byte(strings.TrimSpace(content)), 0644)
}
