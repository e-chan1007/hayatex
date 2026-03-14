package installer

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/e-chan1007/hayatex/internal/config"
	"github.com/e-chan1007/hayatex/internal/resolver"
	"github.com/e-chan1007/hayatex/internal/texconfig"
	"github.com/e-chan1007/hayatex/internal/utils"
)

const (
	decideMirrorJobKey = iota
	parseTLPDBJobKey
	downloadJobKey
	extractJobKey
	addPathJobKey
	genLangConfigJobKey
	genLsRInitialJobKey
	updmapJobKey
	formatTexJobKey
	genLsRFinalJobKey
)

func Install(config *config.Config, tlpdb *resolver.TLDatabase, roots []string, logWriter io.Writer, jobChan chan<- *InstallJobList) error {
	installJobs := InstallJobList{
		Jobs: []InstallJob{
			NewInstallJob(parseTLPDBJobKey, "Parse TeX Live Package Database", false),
			NewInstallJob(downloadJobKey, "Download packages", true),
			NewInstallJob(extractJobKey, "Extract packages ", true),
			NewInstallJob(addPathJobKey, "Add PATH", false),
			NewInstallJob(genLangConfigJobKey, "Generate language config", false),
			NewInstallJob(genLsRInitialJobKey, "Generate ls-R(Initial)", false),
			NewInstallJob(updmapJobKey, "Update font map files", false),
			NewInstallJob(formatTexJobKey, "Format TeX", false),
			NewInstallJob(genLsRFinalJobKey, "Generate ls-R(Final)", false),
		},
		channel: jobChan,
	}

	if config.Arch == "" {
		return fmt.Errorf("Unsupported architecture")
	}

	config.TexDir, _ = filepath.Abs(config.TexDir)

	var err error

	err = utils.CheckWritePermission(config.TexDir)
	if err != nil {
		return fmt.Errorf("No write permission for the installation directory: %v", err)
	}

	installJobs.UpdateJobStatus(parseTLPDBJobKey, InstallJobExecuting)

	deps := resolver.ResolveDependencies(roots, tlpdb, config.Arch)
	if config.Arch == "windows" {
		(*deps)["tlperl.windows"] = tlpdb.Packages["tlperl.windows"]
	}

	dl := NewDownloader(config)
	downloadEstimate := dl.EstimateDownload(deps)
	installJobs.UpdateJobStatusWithMessage(parseTLPDBJobKey, InstallJobCompleted, fmt.Sprintf("TeX Live %s - %d packages(%d files)", tlpdb.Year, len(*deps), downloadEstimate.TotalDownloads))

	installJobs.UpdateJobStatus(downloadJobKey, InstallJobExecuting)
	start := time.Now()
	downloadProgressChan := make(chan *InstallProgress)
	extractProgressChan := make(chan *InstallProgress)
	onProgress := func(job int, format string, progress *InstallProgress) {
		installJobs.UpdateJobStatusWithMessage(job, InstallJobExecuting, fmt.Sprintf(format, progress.Count, progress.TotalCount, utils.FormatBytes(progress.Size, "B"), utils.FormatBytes(progress.TotalSize, "B")))
		progressValue := float64(progress.Count) / float64(progress.TotalCount)
		if progress.Count < progress.TotalCount && progressValue >= 0.99 {
			progressValue = 0.99
		}
		installJobs.UpdateJobProgress(job, progressValue)
	}
	go func() {
		for progress := range downloadProgressChan {
			onProgress(downloadJobKey, "Downloading... (%d/%d, %s/%s)", progress)
		}
	}()
	go func() {
		for progress := range extractProgressChan {
			onProgress(extractJobKey, "Extracting...  (%d/%d, %s/%s)", progress)
		}
	}()

	ctx := context.Background()
	if err := dl.InstallPackages(ctx, deps, downloadProgressChan, extractProgressChan); err != nil {
		return fmt.Errorf("Failed to install packages: %v", err)
	}

	if err := texconfig.SaveLocalTLPDB(config, tlpdb, deps); err != nil {
		return fmt.Errorf("Failed to write texlive.tlpdb: %v", err)
	}
	texconfig.GenerateTexmfConfig(config)
	installJobs.UpdateJobStatusWithMessage(downloadJobKey, InstallJobCompleted, "")
	installJobs.UpdateJobStatusWithMessage(extractJobKey, InstallJobCompleted, fmt.Sprintf("Installed %d packages", len(*deps)))

	texCommandExecutor := utils.TeXCommandExecutor(config, logWriter)

	if config.AddPath {
		installJobs.UpdateJobStatus(addPathJobKey, InstallJobExecuting)
		start = time.Now()
		if err := texconfig.AddSystemLinks(config); err != nil {
			fmt.Fprintf(logWriter, "⚠️ Adding Path failed: %v", err)
			installJobs.UpdateJobStatusWithMessage(addPathJobKey, InstallJobCompleted, "Failed")
		} else {
			fmt.Fprintf(logWriter, "✅ PATH added in %v\n", time.Since(start))
			installJobs.UpdateJobStatus(addPathJobKey, InstallJobCompleted)
		}
	} else {
		installJobs.UpdateJobStatusWithMessage(addPathJobKey, InstallJobCompleted, "Skipped")
	}

	installJobs.UpdateJobStatus(genLangConfigJobKey, InstallJobExecuting)
	start = time.Now()
	if err := texconfig.GenerateLanguageConfig(config.TexDir, deps); err != nil {
		fmt.Fprintf(logWriter, "⚠️ Failed to generate language config: %v", err)
		installJobs.UpdateJobStatusWithMessage(genLangConfigJobKey, InstallJobCompleted, "Failed")
	} else {
		fmt.Fprintf(logWriter, "✅ Generated language config in %v\n", time.Since(start))
		installJobs.UpdateJobStatus(genLangConfigJobKey, InstallJobCompleted)
	}

	installJobs.UpdateJobStatus(genLsRInitialJobKey, InstallJobExecuting)
	start = time.Now()
	texconfig.GenerateLsR(config.TexDir)
	fmt.Fprintf(logWriter, "✅ Generated ls-R in %v\n", time.Since(start))
	installJobs.UpdateJobStatus(genLsRInitialJobKey, InstallJobCompleted)

	installJobs.UpdateJobStatus(updmapJobKey, InstallJobExecuting)
	start = time.Now()
	if err := texconfig.ExecuteUpdmap(config.TexDir, deps); err != nil {
		fmt.Fprintf(logWriter, "⚠️ updmap failed: %v", err)
		installJobs.UpdateJobStatusWithMessage(updmapJobKey, InstallJobCompleted, "Failed")
	} else {
		fmt.Fprintf(logWriter, "✅ updmap completed in %v\n", time.Since(start))
		installJobs.UpdateJobStatus(updmapJobKey, InstallJobCompleted)
	}

	installJobs.UpdateJobStatus(formatTexJobKey, InstallJobExecuting)
	start = time.Now()
	if err := texconfig.ExecuteFormatCommands(ctx, config, deps, &texCommandExecutor); err != nil {
		fmt.Fprintf(logWriter, "⚠️ fmtutil(rewritten) failed: %v", err)
		installJobs.UpdateJobStatusWithMessage(formatTexJobKey, InstallJobCompleted, "Failed")
	}
	fmt.Fprintf(logWriter, "✅ fmtutil(rewritten) completed in %v\n", time.Since(start))
	installJobs.UpdateJobStatus(formatTexJobKey, InstallJobCompleted)

	installJobs.UpdateJobStatus(genLsRFinalJobKey, InstallJobExecuting)
	texconfig.GenerateLsR(config.TexDir)
	installJobs.UpdateJobStatus(genLsRFinalJobKey, InstallJobCompleted)

	return nil
}
