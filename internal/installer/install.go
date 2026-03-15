package installer

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sync"

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
	defer close(jobChan)

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
		return err
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
	downloadProgressChan := make(chan *InstallProgress)
	extractProgressChan := make(chan *InstallProgress)
	var progressWG sync.WaitGroup
	onProgress := func(job int, format func(progress *InstallProgress) string, progressValueFn func(progress *InstallProgress) float64, progress *InstallProgress) {
		installJobs.UpdateJobStatusWithMessage(job, InstallJobExecuting, format(progress))
		progressValue := progressValueFn(progress)
		if progress.Count < progress.TotalCount && progressValue >= 0.99 {
			progressValue = 0.99
		}
		installJobs.UpdateJobProgress(job, progressValue)
	}

	progressWG.Go(func() {
		for progress := range downloadProgressChan {
			onProgress(downloadJobKey, func(p *InstallProgress) string {
				return fmt.Sprintf("Downloading... (%d/%d, %s/%s)", p.Count, p.TotalCount, utils.FormatBytes(p.Size, "B"), utils.FormatBytes(p.TotalSize, "B"))
			}, func(p *InstallProgress) float64 {
				return float64(p.Size) / float64(p.TotalSize)
			}, progress)
		}
	})
	progressWG.Go(func() {
		for progress := range extractProgressChan {
			onProgress(extractJobKey, func(p *InstallProgress) string {
				return fmt.Sprintf("Extracting...  (%d/%d, %s)", p.Count, p.TotalCount, utils.FormatBytes(p.Size, "B"))
			}, func(p *InstallProgress) float64 {
				return float64(p.Count) / float64(p.TotalCount)
			}, progress)
		}
	})

	ctx := context.Background()
	if err := dl.InstallPackages(ctx, deps, downloadProgressChan, extractProgressChan); err != nil {
		progressWG.Wait()
		return fmt.Errorf("Failed to install packages: %v", err)
	}
	progressWG.Wait()

	if err := texconfig.SaveLocalTLPDB(config, tlpdb, deps); err != nil {
		return fmt.Errorf("Failed to write texlive.tlpdb: %v", err)
	}
	texconfig.GenerateTexmfConfig(config)
	installJobs.UpdateJobStatusWithMessage(downloadJobKey, InstallJobCompleted, "")
	installJobs.UpdateJobStatusWithMessage(extractJobKey, InstallJobCompleted, fmt.Sprintf("Installed %d packages", len(*deps)))

	texCommandExecutor := utils.TeXCommandExecutor(config, logWriter)

	if config.AddPath {
		installJobs.UpdateJobStatus(addPathJobKey, InstallJobExecuting)
		if err := texconfig.AddSystemLinks(config); err != nil {
			fmt.Fprintf(logWriter, "⚠️ Adding PATH failed: %v", err)
			installJobs.UpdateJobStatusWithMessage(addPathJobKey, InstallJobCompleted, "Failed")
		} else {
			installJobs.UpdateJobStatus(addPathJobKey, InstallJobCompleted)
		}
	} else {
		installJobs.UpdateJobStatusWithMessage(addPathJobKey, InstallJobSkipped, "Skipped")
	}

	installJobs.UpdateJobStatus(genLangConfigJobKey, InstallJobExecuting)
	if err := texconfig.GenerateLanguageConfig(config.TexDir, deps); err != nil {
		fmt.Fprintf(logWriter, "⚠️ Failed to generate language config: %v", err)
		installJobs.UpdateJobStatusWithMessage(genLangConfigJobKey, InstallJobCompleted, "Failed")
	} else {
		installJobs.UpdateJobStatus(genLangConfigJobKey, InstallJobCompleted)
	}

	installJobs.UpdateJobStatus(genLsRInitialJobKey, InstallJobExecuting)

	texconfig.GenerateLsR(config.TexDir)
	installJobs.UpdateJobStatus(genLsRInitialJobKey, InstallJobCompleted)

	installJobs.UpdateJobStatus(updmapJobKey, InstallJobExecuting)
	if err := texconfig.ExecuteUpdmap(config.TexDir, deps); err != nil {
		fmt.Fprintf(logWriter, "⚠️ fast-updmap failed: %v", err)
		installJobs.UpdateJobStatusWithMessage(updmapJobKey, InstallJobCompleted, "Failed")
	} else {
		installJobs.UpdateJobStatus(updmapJobKey, InstallJobCompleted)
	}

	installJobs.UpdateJobStatus(formatTexJobKey, InstallJobExecuting)
	if err := texconfig.ExecuteFormatCommands(ctx, config, deps, &texCommandExecutor); err != nil {
		fmt.Fprintf(logWriter, "⚠️ fast-fmtutil failed: %v", err)
		installJobs.UpdateJobStatusWithMessage(formatTexJobKey, InstallJobCompleted, "Failed")
	}
	installJobs.UpdateJobStatus(formatTexJobKey, InstallJobCompleted)

	installJobs.UpdateJobStatus(genLsRFinalJobKey, InstallJobExecuting)
	texconfig.GenerateLsR(config.TexDir)
	installJobs.UpdateJobStatus(genLsRFinalJobKey, InstallJobCompleted)

	return nil
}
