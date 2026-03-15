package installer

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
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
	pkg                    *resolver.TLPackage
	label                  string
	data                   []byte
	estimatedExtractedSize uint64
}

type Downloader struct {
	maxDownloadWorkers       int
	maxExtractWorkers        int
	maxRetryCount            int
	heavyFileWorkerRatio     float64
	minSplitSize             uint64
	chunkSize                uint64
	client                   *http.Client
	config                   *config.Config
	downloadJobs             []downloadJob
	connectionSemaphore      chan struct{}
	downloadProgress         *progressCounter
	extractProgress          *progressCounter
	downloadProgressChan     chan<- *InstallProgress
	extractProgressChan      chan<- *InstallProgress
	createdDirs              sync.Map
	disableParallelDownloads atomic.Bool
}

type PackageTracker struct {
	pkg             *resolver.TLPackage
	url             string
	data            []byte
	remainingChunks atomic.Uint32
	totalChunks     atomic.Uint32
	downloadedSize  atomic.Uint64
	totalSize       atomic.Uint64
	completed       atomic.Bool
	errOnce         sync.Once
	firstErr        error
	ctx             context.Context
	cancel          context.CancelFunc
	retryCount      atomic.Uint32
	wg              *sync.WaitGroup
}

type ChunkTask struct {
	tracker  *PackageTracker
	url      string
	offset   uint64
	size     uint64
	isSingle bool
}

func NewDownloader(config *config.Config) *Downloader {
	maxConcurrentConnections := 10
	tr := &http.Transport{
		ResponseHeaderTimeout: 30 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		IdleConnTimeout:       120 * time.Second,
		MaxIdleConns:          maxConcurrentConnections,
		MaxIdleConnsPerHost:   maxConcurrentConnections,
		MaxConnsPerHost:       maxConcurrentConnections,
	}
	return &Downloader{
		maxDownloadWorkers:  runtime.NumCPU(),
		maxExtractWorkers:   runtime.NumCPU(),
		maxRetryCount:       5,
		minSplitSize:        20 * 1024 * 1024, // 20 MB
		chunkSize:           10 * 1024 * 1024, // 10 MB
		client:              &http.Client{Transport: tr, Timeout: 60 * time.Second},
		config:              config,
		connectionSemaphore: make(chan struct{}, maxConcurrentConnections),
		downloadProgress:    &progressCounter{},
		extractProgress:     &progressCounter{},
	}
}

type InstallEstimate struct {
	TotalDownloads     int
	TotalDownloadSize  uint64
	TotalExtractedSize uint64
}

type InstallProgress struct {
	Count      uint32
	Size       uint64
	TotalCount uint32
	TotalSize  uint64
}

type progressCounter struct {
	Count      atomic.Uint32
	Size       atomic.Uint64
	TotalCount atomic.Uint32
	TotalSize  atomic.Uint64
}

func (d *Downloader) EstimateDownload(packages *resolver.TLPackageList) InstallEstimate {
	if len(d.downloadJobs) == 0 {
		for _, pkg := range *packages {
			if pkg.Container.Size > 0 {
				url, _ := url.JoinPath(d.config.MirrorURL, fmt.Sprintf("systems/texlive/tlnet/archive/%s.tar.xz", pkg.Name))
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
				url, _ := url.JoinPath(d.config.MirrorURL, fmt.Sprintf("systems/texlive/tlnet/archive/%s-doc.tar.xz", pkg.Name))
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
				url, _ := url.JoinPath(d.config.MirrorURL, fmt.Sprintf("systems/texlive/tlnet/archive/%s-src.tar.xz", pkg.Name))
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
	extractedSize <<= 10
	d.downloadProgress.TotalCount.Store(uint32(len(d.downloadJobs)))
	d.downloadProgress.TotalSize.Store(totalSize)
	d.extractProgress.TotalCount.Store(uint32(len(d.downloadJobs)))
	d.extractProgress.TotalSize.Store(extractedSize)
	return InstallEstimate{
		TotalDownloads:     len(d.downloadJobs),
		TotalDownloadSize:  totalSize,
		TotalExtractedSize: extractedSize,
	}
}

func (d *Downloader) InstallPackages(ctx context.Context, packages *resolver.TLPackageList, downloadProgressChan chan<- *InstallProgress, extractProgressChan chan<- *InstallProgress) error {
	d.downloadProgressChan = downloadProgressChan
	d.extractProgressChan = extractProgressChan
	d.EstimateDownload(packages)

	downloadTaskChan := make(chan ChunkTask, d.maxDownloadWorkers*2)
	extractJobChan := make(chan extractJob, d.maxDownloadWorkers*2)
	var wg sync.WaitGroup
	wg.Go(func() {
		d.createChunkTasks(ctx, downloadTaskChan, &wg)
	})

	eg, ctx := errgroup.WithContext(ctx)
	for i := 0; i < d.maxDownloadWorkers; i++ {
		eg.Go(func() error {
			return d.chunkTaskWorker(ctx, downloadTaskChan, extractJobChan)
		})
	}

	go func() {
		wg.Wait()
		close(downloadTaskChan)
		close(extractJobChan)
	}()

	for i := 0; i < d.maxExtractWorkers; i++ {
		eg.Go(func() error {
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case job, ok := <-extractJobChan:
					if !ok {
						return nil
					}
					extractedSize, err := d.extract(ctx, &job)
					if err != nil {
						return fmt.Errorf("failed to extract %s: %w", job.label, err)
					}
					if job.estimatedExtractedSize < extractedSize {
						d.extractProgress.TotalSize.Add(extractedSize - job.estimatedExtractedSize)
					}
					d.extractProgress.Size.Add(extractedSize)
					d.extractProgress.Count.Add(1)
					select {
					case d.extractProgressChan <- d.extractProgress.Snapshot():
					default:
					}
				}
			}
		})
	}

	err := eg.Wait()
	downloadProgressChan <- d.downloadProgress.Completed()
	extractProgressChan <- d.extractProgress.Completed()
	close(downloadProgressChan)
	close(extractProgressChan)
	return err
}

func NewPackageTracker(parentCtx context.Context, pkg *resolver.TLPackage, url string, size uint64, chunks uint32, wg *sync.WaitGroup) *PackageTracker {
	ctx, cancel := context.WithCancel(parentCtx)

	var data []byte
	if size > 0 {
		data = make([]byte, size)
	}

	tracker := &PackageTracker{
		pkg:    pkg,
		url:    url,
		data:   data,
		ctx:    ctx,
		cancel: cancel,
		wg:     wg,
	}
	tracker.remainingChunks.Store(chunks)
	tracker.totalChunks.Store(chunks)
	tracker.totalSize.Store(size)
	return tracker
}

func (tracker *PackageTracker) abort(err error) {
	tracker.errOnce.Do(func() {
		tracker.firstErr = err
		tracker.cancel()
	})
}

func (d *Downloader) createChunkTasks(ctx context.Context, taskChan chan ChunkTask, wg *sync.WaitGroup) {
	for _, job := range sortDownloadJobs(d.downloadJobs, d.maxDownloadWorkers) {
		chunks := uint32(1)
		if job.size >= d.minSplitSize {
			chunks = uint32(math.Ceil(float64(job.size) / float64(d.chunkSize)))
		}
		tracker := NewPackageTracker(ctx, job.pkg, job.url, job.size, chunks, wg)
		d.enqueuePackage(ctx, tracker, taskChan, wg)
	}
}

func (d *Downloader) enqueuePackage(ctx context.Context, tracker *PackageTracker, taskChan chan ChunkTask, wg *sync.WaitGroup) {
	totalChunks := tracker.totalChunks.Load()
	tracker.wg = wg
	tracker.downloadedSize.Store(0)
	tracker.remainingChunks.Store(totalChunks)
	wg.Add(int(totalChunks))

	for i := range totalChunks {
		start := uint64(i) * d.chunkSize
		chunkSize := d.chunkSize
		totalSize := tracker.totalSize.Load()
		if start+chunkSize > totalSize {
			chunkSize = totalSize - start
		}

		select {
		case <-ctx.Done():
			return
		case taskChan <- ChunkTask{
			tracker: tracker, url: tracker.url, offset: start, size: chunkSize,
			isSingle: totalSize < d.minSplitSize,
		}:
		}
	}
}

func (d *Downloader) chunkTaskWorker(ctx context.Context, taskChan chan ChunkTask, extractJobChan chan<- extractJob) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case task, ok := <-taskChan:
			if !ok {
				return nil
			}
			err := d.handleChunkTask(ctx, task, taskChan, extractJobChan)
			task.tracker.wg.Done()
			if err != nil {
				return err
			}
		}
	}
}

func (d *Downloader) handleChunkTask(ctx context.Context, task ChunkTask, taskChan chan ChunkTask, extractJobChan chan<- extractJob) error {
	tracker := task.tracker

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-tracker.ctx.Done():
		return tracker.ctx.Err()
	default:
	}

	var data []byte
	var err error
	for retryCount := range d.maxRetryCount {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d.connectionSemaphore <- struct{}{}:
			defer func() { <-d.connectionSemaphore }()
		}
		data, err = d.fetchChunk(ctx, task)
		if err == nil {
			if !task.isSingle && uint64(len(data)) >= tracker.totalSize.Load() {
				if tracker.completed.CompareAndSwap(false, true) {
					return nil
				}
				tracker.cancel()
				task.isSingle = true
				tracker.remainingChunks.Store(1)
				tracker.totalSize.Store(0)
			}
			break
		}
		time.Sleep(time.Duration(1<<retryCount) * time.Second)
	}
	if err != nil {
		tracker.abort(fmt.Errorf("failed to download %s: %w", tracker.pkg.Name, err))
		return err
	}

	if task.isSingle {
		tracker.data = data
	} else {
		copy(tracker.data[task.offset:], data)
	}
	d.downloadProgress.Size.Add(uint64(len(data)))
	tracker.downloadedSize.Add(uint64(len(data)))

	if tracker.remainingChunks.Add(^uint32(0)) == 0 {
		if tracker.firstErr != nil {
			return tracker.firstErr
		}
		err := verifyChecksum(tracker.data, tracker.pkg.Container.Checksum)
		if err != nil {
			if tracker.retryCount.Add(1) <= uint32(d.maxRetryCount) {
				d.downloadProgress.Size.Add(^tracker.downloadedSize.Load() + 1)
				tracker.totalChunks.Store(0)
				tracker.wg.Go(func() {
					d.enqueuePackage(ctx, tracker, taskChan, tracker.wg)
				})
				return nil
			}
			tracker.abort(fmt.Errorf("checksum mismatch for %s after %d retries", tracker.pkg.Name, d.maxRetryCount))
			return tracker.firstErr
		}
		d.downloadProgress.Count.Add(1)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case extractJobChan <- extractJob{pkg: tracker.pkg, label: tracker.pkg.Name, data: tracker.data, estimatedExtractedSize: tracker.totalSize.Load()}:
		}
	}
	select {
	case d.downloadProgressChan <- d.downloadProgress.Snapshot():
	default:
	}
	return nil
}

func (d *Downloader) fetchChunk(ctx context.Context, task ChunkTask) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", task.url, nil)
	if err != nil {
		return nil, err
	}
	if !task.isSingle && !d.disableParallelDownloads.Load() {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", task.offset, task.offset+task.size-1))
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}
	if !task.isSingle && resp.StatusCode == http.StatusOK {
		d.disableParallelDownloads.Store(true)
	}
	var data []byte
	if resp.ContentLength >= 0 {
		data = make([]byte, resp.ContentLength)
		_, err = io.ReadFull(resp.Body, data)
		return data, err
	}
	return io.ReadAll(resp.Body)
}

func verifyChecksum(data []byte, expected string) error {
	sum := sha512.Sum512(data)
	if hex.EncodeToString(sum[:]) != expected {
		return fmt.Errorf("checksum mismatch")
	}
	return nil
}

func (p *progressCounter) Snapshot() *InstallProgress {
	return &InstallProgress{
		Count:      p.Count.Load(),
		Size:       p.Size.Load(),
		TotalCount: p.TotalCount.Load(),
		TotalSize:  p.TotalSize.Load(),
	}
}

func (p *progressCounter) Completed() *InstallProgress {
	return &InstallProgress{
		Count:      p.TotalCount.Load(),
		Size:       p.TotalSize.Load(),
		TotalCount: p.TotalCount.Load(),
		TotalSize:  p.TotalSize.Load(),
	}
}

func (d *Downloader) extract(ctx context.Context, job *extractJob) (uint64, error) {
	format := archives.CompressedArchive{
		Compression: archives.Xz{},
		Extraction:  archives.Tar{},
	}

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
		if _, created := d.createdDirs.LoadOrStore(dir, struct{}{}); !created {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
		}

		if f.LinkTarget != "" {
			os.RemoveAll(targetPath)
			return os.Symlink(f.LinkTarget, targetPath)
		}

		out, err := os.OpenFile(targetPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		writer := bufio.NewWriterSize(out, 64*1024)

		written, err := io.Copy(writer, rc)
		writer.Flush()
		out.Close()

		extractedSize += uint64(written)
		return err
	})
	return extractedSize, err
}

func sortDownloadJobs(jobs []downloadJob, workerCount int) []downloadJob {
	simpleSortedJobs := make([]downloadJob, len(jobs))
	copy(simpleSortedJobs, jobs)
	slices.SortFunc(simpleSortedJobs, func(a, b downloadJob) int {
		return int(b.size) - int(a.size)
	})
	totalSize := uint64(0)
	for _, job := range simpleSortedJobs {
		totalSize += job.size
	}
	sortedJobs := make([]downloadJob, len(jobs))
	largeJobs := []downloadJob{}
	smallJobs := []downloadJob{}
	largeJobsTotalSize := float64(0)
	largeJobsThreshold := float64(totalSize) * 0.8

	for _, job := range simpleSortedJobs {
		if largeJobsTotalSize < largeJobsThreshold {
			largeJobs = append(largeJobs, job)
			largeJobsTotalSize += float64(job.size)
		} else {
			smallJobs = append(smallJobs, job)
		}
	}

	ratio := max(math.Ceil(float64(len(largeJobs))/float64(len(jobs))), float64(workerCount))

	i, j, k := 0, 0, 0
	for k < len(jobs) {
		for r := 0; r < int(ratio) && i < len(smallJobs); r++ {
			sortedJobs[k] = smallJobs[i]
			i++
			k++
		}
		if j < len(largeJobs) {
			sortedJobs[k] = largeJobs[j]
			j++
			k++
		}
	}
	return sortedJobs
}
