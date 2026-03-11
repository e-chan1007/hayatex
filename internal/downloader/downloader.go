package downloader

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/e-chan1007/hayatex/internal/resolver"
	"github.com/mholt/archives"
	"golang.org/x/sync/errgroup"
)

type pkgJob struct {
	pkg  *resolver.TLPackage
	data []byte
}

type Downloader struct {
	BaseURL    string
	TargetDir  string
	MaxWorkers int
	client     *http.Client
}

func New(baseURL, targetDir string) *Downloader {
	tr := &http.Transport{
		TLSHandshakeTimeout: 30 * time.Second,
		IdleConnTimeout:     90 * time.Second,
		// コネクションプールをWorkersより多めに確保して、
		// 展開中のハンドシェイク待ちを解消する
		MaxIdleConnsPerHost: 50,
		MaxIdleConns:        100,
		ReadBufferSize:      128 * 1024, // 128KB
		WriteBufferSize:     128 * 1024,
	}
	return &Downloader{
		BaseURL:    baseURL,
		TargetDir:  targetDir,
		MaxWorkers: 16, // CPUコア数に合わせて少し多めに
		client:     &http.Client{Transport: tr},
	}
}

func (d *Downloader) InstallPackages(ctx context.Context, pkgs map[string]*resolver.TLPackage) error {
	eg, ctx := errgroup.WithContext(ctx)

	// パッケージリストのフィルタリング
	var taskList []*resolver.TLPackage
	for _, p := range pkgs {
		if p.Container.Size > 0 {
			taskList = append(taskList, p)
		}
	}
	total := len(taskList)
	var completed int32

	// ダウンロード済みデータを展開Workerに渡すパイプライン
	jobChan := make(chan pkgJob, d.MaxWorkers*2)

	// --- Step 1: ダウンロードWorker群 (I/O Bound) ---
	// 展開より速く終わるように、ダウンロードの並列度を確保
	downloadGroup, dlCtx := errgroup.WithContext(ctx)
	dlSem := make(chan struct{}, d.MaxWorkers) // 同時ダウンロード数

	eg.Go(func() error {
		defer close(jobChan)
		for _, p := range taskList {
			p := p
			select {
			case <-dlCtx.Done():
				return dlCtx.Err()
			case dlSem <- struct{}{}:
				downloadGroup.Go(func() error {
					defer func() { <-dlSem }()
					data, err := d.download(dlCtx, p)
					if err != nil {
						return err
					}
					jobChan <- pkgJob{pkg: p, data: data}
					return nil
				})
			}
		}
		return downloadGroup.Wait()
	})

	// --- Step 2: 展開Worker群 (CPU Bound) ---
	// XZ展開は重いので、ここも並列で回す
	for i := 0; i < d.MaxWorkers; i++ {
		eg.Go(func() error {
			for job := range jobChan {
				if err := d.extract(ctx, job.pkg, job.data); err != nil {
					return err
				}
				curr := atomic.AddInt32(&completed, 1)
				// 10個おきに表示して画面更新のI/Oを節約
				if curr%10 == 0 || int(curr) == total {
					fmt.Printf("[%d/%d] %s\n", curr, total, job.pkg.Name)
				}
			}
			return nil
		})
	}

	return eg.Wait()
}

func (d *Downloader) download(ctx context.Context, pkg *resolver.TLPackage) ([]byte, error) {
	url := d.BaseURL + "archive/" + pkg.Name + ".tar.xz"

	var lastErr error
	for i := range 3 {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
		case <-time.After(time.Duration(i+1) * time.Second):
		}
	}
	return nil, fmt.Errorf("[%s] download failed: %v", pkg.Name, lastErr)
}

func (d *Downloader) extract(ctx context.Context, pkg *resolver.TLPackage, data []byte) error {
	format := archives.CompressedArchive{
		Compression: archives.Xz{},
		Extraction:  archives.Tar{},
	}

	lastDir := ""
	return format.Extract(ctx, bytes.NewReader(data), func(ctx context.Context, f archives.FileInfo) error {
		var targetPath string
		if pkg.Relocated {
			targetPath = filepath.Join(d.TargetDir, "texmf-dist", f.NameInArchive)
		} else {
			targetPath = filepath.Join(d.TargetDir, f.NameInArchive)
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

		// ファイル書き出し
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

		_, err = io.Copy(out, rc)
		return err
	})
}
