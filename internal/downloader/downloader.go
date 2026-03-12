package downloader

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/e-chan1007/hayatex/internal/config"
	"github.com/e-chan1007/hayatex/internal/resolver"
	"github.com/mholt/archives"
	"golang.org/x/sync/errgroup"
)

type downloadJob struct {
	pkg           *resolver.TLPackage
	label         string
	url           string
	checksum      string
	size          uint64
	extractedSize uint64
}

type extractJob struct {
	pkg   *resolver.TLPackage
	label string
	data  []byte
}

type Downloader struct {
	maxWorkers   int
	client       *http.Client
	config       *config.Config
	downloadJobs []downloadJob
}

func New(config *config.Config) *Downloader {
	tr := &http.Transport{
		TLSHandshakeTimeout: 30 * time.Second,
		IdleConnTimeout:     90 * time.Second,
		MaxIdleConnsPerHost: 50,
		MaxIdleConns:        100,
		ReadBufferSize:      128 * 1024,
		WriteBufferSize:     128 * 1024,
	}
	return &Downloader{
		maxWorkers: 16,
		client:     &http.Client{Transport: tr},
		config:     config,
	}
}

type InstallEstimate struct {
	TotalDownloads     int
	TotalDownloadSize  uint64
	TotalExtractedSize uint64
}

type InstallProgress struct {
	CompletedCount uint32
	DownloadedSize uint64
	ExtractedSize  uint64
}

func (d *Downloader) EstimateDownload(packages *resolver.TLDatabase) InstallEstimate {
	if len(d.downloadJobs) == 0 {
		for _, pkg := range *packages {
			if pkg.Container.Size > 0 {
				url, _ := url.JoinPath(d.config.MirrorURL, fmt.Sprintf("archive/%s.tar.xz", pkg.Name))
				extractedSize := pkg.RunFiles.Size
				if binFiles, ok := pkg.BinFiles[d.config.Arch]; ok {
					extractedSize += binFiles.Size
				}
				d.downloadJobs = append(d.downloadJobs, downloadJob{
					pkg:           pkg,
					label:         pkg.Name,
					url:           url,
					checksum:      pkg.Container.Checksum,
					size:          pkg.Container.Size,
					extractedSize: extractedSize,
				})
			}
			if d.config.InstallDocFiles && pkg.DocContainer.Size > 0 {
				url, _ := url.JoinPath(d.config.MirrorURL, fmt.Sprintf("archive/%s-doc.tar.xz", pkg.Name))
				d.downloadJobs = append(d.downloadJobs, downloadJob{
					pkg:           pkg,
					label:         pkg.Name + "-doc",
					url:           url,
					checksum:      pkg.DocContainer.Checksum,
					size:          pkg.DocContainer.Size,
					extractedSize: pkg.DocFiles.Size,
				})
			}
			if d.config.InstallSrcFiles && pkg.SrcContainer.Size > 0 {
				url, _ := url.JoinPath(d.config.MirrorURL, fmt.Sprintf("archive/%s-src.tar.xz", pkg.Name))
				d.downloadJobs = append(d.downloadJobs, downloadJob{
					pkg:           pkg,
					label:         pkg.Name + "-src",
					url:           url,
					checksum:      pkg.SrcContainer.Checksum,
					size:          pkg.SrcContainer.Size,
					extractedSize: pkg.SrcFiles.Size,
				})
			}
		}
	}
	totalSize := uint64(0)
	extractedSize := uint64(0)
	for _, job := range d.downloadJobs {
		totalSize += job.size
		extractedSize += job.extractedSize
	}
	return InstallEstimate{
		TotalDownloads:     len(d.downloadJobs),
		TotalDownloadSize:  totalSize,
		TotalExtractedSize: extractedSize << 10, // KB to B
	}
}

func (d *Downloader) InstallPackages(ctx context.Context, packages *resolver.TLDatabase, progressChan chan<- *InstallProgress) error {
	d.EstimateDownload(packages)
	progress := &InstallProgress{CompletedCount: 0, DownloadedSize: 0, ExtractedSize: 0}

	eg, ctx := errgroup.WithContext(ctx)
	downloadGroup, dlCtx := errgroup.WithContext(ctx)
	jobChan := make(chan extractJob, d.maxWorkers*2)
	dlSem := make(chan struct{}, d.maxWorkers)

	eg.Go(func() error {
		defer close(jobChan)
		for _, job := range d.downloadJobs {
			select {
			case <-dlCtx.Done():
				return dlCtx.Err()
			case dlSem <- struct{}{}:
				downloadGroup.Go(func() error {
					defer func() { <-dlSem }()
					data, err := d.download(dlCtx, &job)
					if err != nil {
						return err
					}
					atomic.AddUint64(&progress.DownloadedSize, job.size)
					jobChan <- extractJob{pkg: job.pkg, label: job.label, data: data}
					return nil
				})
			}
		}
		return downloadGroup.Wait()
	})

	for i := 0; i < d.maxWorkers; i++ {
		eg.Go(func() error {
			for job := range jobChan {
				extractedSize, err := d.extract(ctx, &job)
				if err != nil {
					return err
				}
				atomic.AddUint32(&progress.CompletedCount, 1)
				atomic.AddUint64(&progress.ExtractedSize, extractedSize)

				progressChan <- progress
			}
			return nil
		})
	}

	err := eg.Wait()
	close(progressChan)
	return err
}

func (d *Downloader) download(ctx context.Context, task *downloadJob) ([]byte, error) {
	var lastErr error

	for i := range 3 {
		req, err := http.NewRequestWithContext(ctx, "GET", task.url, nil)
		if err != nil {
			return nil, err
		}

		resp, err := d.client.Do(req)
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				data, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err == nil {
					return data, nil
				}
				lastErr = err
			} else {
				resp.Body.Close()
				lastErr = fmt.Errorf("status: %s", resp.Status)
			}
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(1<<(i+1)) * time.Second):
		}
	}
	return nil, lastErr
}

func (d *Downloader) extract(ctx context.Context, job *extractJob) (uint64, error) {
	format := archives.CompressedArchive{
		Compression: archives.Xz{},
		Extraction:  archives.Tar{},
	}

	lastDir := ""
	extractedSize := uint64(0)
	err := format.Extract(ctx, bytes.NewReader(job.data), func(ctx context.Context, f archives.FileInfo) error {
		var targetPath string
		if job.pkg.Relocated {
			targetPath = filepath.Join(d.config.TexDir, "texmf-dist", f.NameInArchive)
		} else {
			targetPath = filepath.Join(d.config.TexDir, f.NameInArchive)
		}

		if f.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		dir := filepath.Dir(targetPath)
		if dir != lastDir {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
			lastDir = dir
		}

		if f.LinkTarget != "" {
			os.RemoveAll(targetPath)
			return os.Symlink(f.LinkTarget, targetPath)
		}

		out, err := os.OpenFile(targetPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		defer out.Close()

		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		written, err := io.Copy(out, rc)
		extractedSize += uint64(written)

		return err
	})
	return extractedSize, err
}
