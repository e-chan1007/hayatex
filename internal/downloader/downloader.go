package downloader

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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
	retryCount    int8
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
		MaxIdleConns:        runtime.NumCPU() * 4,
	}
	return &Downloader{
		maxWorkers: runtime.NumCPU(),
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
					retryCount:    0,
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
					retryCount:    0,
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
					retryCount:    0,
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
	progress := &InstallProgress{}

	taskChan := make(chan downloadJob, len(d.downloadJobs)*2)
	jobChan := make(chan extractJob, d.maxWorkers*2)

	var pending int32 = int32(len(d.downloadJobs))

	for _, job := range sortDownloadJobs(d.downloadJobs) {
		taskChan <- job
	}

	eg, ctx := errgroup.WithContext(ctx)

	for i := 0; i < d.maxWorkers; i++ {
		eg.Go(func() error {
			for job := range taskChan {
				data, err := d.download(ctx, &job)
				if err != nil {
					if job.retryCount < 3 {
						job.retryCount++
						go func(j downloadJob) {
							time.Sleep(time.Duration(1<<j.retryCount) * time.Second)
							taskChan <- j
						}(job)
						continue
					}
					return fmt.Errorf("failed %s: %w", job.label, err)
				}
				atomic.AddUint64(&progress.DownloadedSize, job.size)
				jobChan <- extractJob{pkg: job.pkg, label: job.label, data: data}

				if atomic.AddInt32(&pending, -1) == 0 {
					close(taskChan)
					close(jobChan)
				}
			}
			return nil
		})
	}

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
	req, err := http.NewRequestWithContext(ctx, "GET", task.url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if task.checksum != "" {
		sum := sha512.Sum512(data)
		actual := hex.EncodeToString(sum[:])
		if actual != task.checksum {
			return nil, fmt.Errorf("checksum mismatch for %s: expected %s, got %s", task.label, task.checksum, actual)
		}
	}

	return data, nil
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

func sortDownloadJobs(jobs []downloadJob) []downloadJob {
	simpleSortedJobs := make([]downloadJob, len(jobs))
	copy(simpleSortedJobs, jobs)
	slices.SortFunc(simpleSortedJobs, func(a, b downloadJob) int {
		return int(b.size) - int(a.size)
	})
	sortedJobs := make([]downloadJob, len(jobs))
	left, right := 0, len(jobs)-1
	for i, v := range simpleSortedJobs {
		if i%2 == 0 {
			sortedJobs[left] = v
			left++
		} else {
			sortedJobs[right] = v
			right--
		}
	}
	return sortedJobs
}
