package installer

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/e-chan1007/hayatex/internal/config"
	"github.com/e-chan1007/hayatex/internal/downloader"
	"github.com/e-chan1007/hayatex/internal/resolver"
	"github.com/e-chan1007/hayatex/internal/texconfig"
	"github.com/e-chan1007/hayatex/internal/utils"
)

const (
	decideMirrorJob  = "decide_mirror"
	parseTLPDBJob    = "parse_tlpdb"
	downloadJob      = "download_packages"
	addPathJob       = "add_path"
	genLangConfigJob = "gen_lang_config"
	genLsRInitialJob = "gen_lsr_initial"
	formatTexJob     = "format_tex"
	updmapJob        = "updmap"
	genLsRFinalJob   = "gen_lsr_final"
)

func Install(config *config.Config, roots []string, logWriter io.Writer, jobChan chan<- *InstallJobList) error {
	installJobs := InstallJobList{
		Jobs: []InstallJob{
			NewInstallJob(decideMirrorJob, "Decide mirror", false),
			NewInstallJob(parseTLPDBJob, "Parse TeX Live Package Database", false),
			NewInstallJob(downloadJob, "Download packages", true),
			NewInstallJob(addPathJob, "Adding PATH", false),
			NewInstallJob(genLangConfigJob, "Generating language config", false),
			NewInstallJob(genLsRInitialJob, "Generating ls-R(Initial)", false),
			NewInstallJob(formatTexJob, "Formatting TeX", false),
			NewInstallJob(updmapJob, "Updating font map files", false),
			NewInstallJob(genLsRFinalJob, "Generating ls-R(Final)", false),
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

	installJobs.UpdateJobStatus(decideMirrorJob, InstallJobExecuting)
	if config.MirrorURL, err = utils.ResolveMirror(config.MirrorURL); err != nil {
		return fmt.Errorf("Failed to resolve mirror URL: %v", err)
	}
	installJobs.UpdateJobStatusWithMessage(decideMirrorJob, InstallJobCompleted, config.MirrorURL)

	installJobs.UpdateJobStatus(parseTLPDBJob, InstallJobExecuting)
	tlpdb, err := resolver.RetrieveTLDatabase(config.MirrorURL)
	if err != nil {
		return fmt.Errorf("Failed to retrieve TLDatabase: %v", err)
	}

	deps := resolver.ResolveDependencies(roots, tlpdb, config.Arch)
	if config.Arch == "windows" {
		(*deps)["tlperl.windows"] = (*tlpdb)["tlperl.windows"]
	}

	dl := downloader.New(config)
	downloadEstimate := dl.EstimateDownload(deps)
	installJobs.UpdateJobStatusWithMessage(parseTLPDBJob, InstallJobCompleted, fmt.Sprintf("%d packages(%d files)", len(*deps), downloadEstimate.TotalDownloads))

	installJobs.UpdateJobStatus(downloadJob, InstallJobExecuting)
	start := time.Now()
	progressChan := make(chan *downloader.InstallProgress)
	go func() {
		for progress := range progressChan {
			installJobs.UpdateJobStatusWithMessage(downloadJob, InstallJobExecuting, fmt.Sprintf("%d / %d downloaded", progress.CompletedCount, downloadEstimate.TotalDownloads))
			progressValue := float64(progress.CompletedCount) / float64(downloadEstimate.TotalDownloads)
			if progress.CompletedCount < uint32(downloadEstimate.TotalDownloads) && progressValue >= 0.99 {
				progressValue = 0.99
			}
			installJobs.UpdateJobProgress(downloadJob, progressValue)
		}
	}()

	ctx := context.Background()
	if err := dl.InstallPackages(ctx, deps, progressChan); err != nil {
		return fmt.Errorf("Failed to install packages: %v", err)
	}

	if err := texconfig.SaveLocalTLPDB(config, tlpdb, deps); err != nil {
		return fmt.Errorf("Failed to write texlive.tlpdb: %v", err)
	}
	installJobs.UpdateJobStatusWithMessage(downloadJob, InstallJobCompleted, fmt.Sprintf("Downloaded %d packages", len(*deps)))

	texCommandExecutor := utils.TeXCommandExecutor(config, logWriter)

	if config.AddPath {
		installJobs.UpdateJobStatus(addPathJob, InstallJobExecuting)
		start = time.Now()
		if err := texconfig.AddSystemLinks(config); err != nil {
			fmt.Fprintf(logWriter, "⚠️ Adding Path failed: %v", err)
			installJobs.UpdateJobStatusWithMessage(addPathJob, InstallJobCompleted, "Failed")
		} else {
			fmt.Fprintf(logWriter, "✅ PATH added in %v\n", time.Since(start))
			installJobs.UpdateJobStatus(addPathJob, InstallJobCompleted)
		}
	} else {
		installJobs.UpdateJobStatusWithMessage(addPathJob, InstallJobCompleted, "Skipped")
	}

	installJobs.UpdateJobStatus(genLangConfigJob, InstallJobExecuting)
	start = time.Now()
	if err := texconfig.GenerateLanguageConfig(config.TexDir, deps); err != nil {
		fmt.Fprintf(logWriter, "⚠️ Failed to generate language config: %v", err)
		installJobs.UpdateJobStatusWithMessage(genLangConfigJob, InstallJobCompleted, "Failed")
	} else {
		fmt.Fprintf(logWriter, "✅ Generated language config in %v\n", time.Since(start))
		installJobs.UpdateJobStatus(genLangConfigJob, InstallJobCompleted)
	}

	installJobs.UpdateJobStatus(genLsRInitialJob, InstallJobExecuting)
	start = time.Now()
	texconfig.GenerateLsR(config.TexDir)
	fmt.Fprintf(logWriter, "✅ Generated ls-R in %v\n", time.Since(start))
	installJobs.UpdateJobStatus(genLsRInitialJob, InstallJobCompleted)

	installJobs.UpdateJobStatus(formatTexJob, InstallJobExecuting)
	start = time.Now()
	texconfig.ExecuteFormatCommands(ctx, config, deps, &texCommandExecutor)
	fmt.Fprintf(logWriter, "✅ fmtutil(rewritten) completed in %v\n", time.Since(start))
	installJobs.UpdateJobStatus(formatTexJob, InstallJobCompleted)

	installJobs.UpdateJobStatus(updmapJob, InstallJobExecuting)
	start = time.Now()
	if err := texconfig.ExecuteUpdmap(config.TexDir, deps); err != nil {
		fmt.Fprintf(logWriter, "⚠️ updmap failed: %v", err)
		installJobs.UpdateJobStatusWithMessage(updmapJob, InstallJobCompleted, "Failed")
	} else {
		fmt.Fprintf(logWriter, "✅ updmap completed in %v\n", time.Since(start))
		installJobs.UpdateJobStatus(updmapJob, InstallJobCompleted)
	}

	installJobs.UpdateJobStatus(genLsRFinalJob, InstallJobExecuting)
	texconfig.GenerateLsR(config.TexDir)
	installJobs.UpdateJobStatus(genLsRFinalJob, InstallJobCompleted)

	return nil
}
