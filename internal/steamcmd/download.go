package steamcmd

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/Wakanda1519/skindl/internal/ui"
)

const (
	rustAppID      = "252490"
	steamcmdZipURL = "https://steamcdn-a.akamaihd.net/client/installer/steamcmd.zip"
)

type Command struct {
	Exe   string
	Dir   string
	Fresh bool
}

func steamcmdExeName() string {
	if runtime.GOOS == "windows" {
		return "steamcmd.exe"
	}
	return "steamcmd.sh"
}

func (c *Command) contentDir() string {
	return filepath.Join(c.Dir, "steamapps", "workshop", "content", rustAppID)
}

func Find(baseDir, override string) (*Command, error) {
	if override != "" {
		abs, err := filepath.Abs(override)
		if err != nil {
			return nil, err
		}
		if st, err := os.Stat(abs); err != nil || st.IsDir() {
			return nil, fmt.Errorf("steamcmd не найден по пути %s", abs)
		}
		return &Command{Exe: abs, Dir: filepath.Dir(abs)}, nil
	}

	candidates := []string{
		filepath.Join(baseDir, "steamcmd", steamcmdExeName()),
		filepath.Join(baseDir, ".steamcmd", steamcmdExeName()),
	}
	if p, err := exec.LookPath(steamcmdExeName()); err == nil {
		candidates = append(candidates, p)
	}
	for _, path := range candidates {
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return &Command{Exe: path, Dir: filepath.Dir(path)}, nil
		}
	}

	ui.Warn("steamcmd не найден, скачиваем официальную сборку с серверов Valve...")
	dir := filepath.Join(baseDir, "steamcmd")
	if err := installSteamcmd(dir); err != nil {
		return nil, err
	}
	return &Command{Exe: filepath.Join(dir, steamcmdExeName()), Dir: dir, Fresh: true}, nil
}

func FilterPending(destDir string, ids []uint64) (pending []uint64, already int) {
	for _, id := range ids {
		if dirHasContent(filepath.Join(destDir, strconv.FormatUint(id, 10))) {
			already++
			continue
		}
		pending = append(pending, id)
	}
	return pending, already
}

func installSteamcmd(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	zipPath := filepath.Join(dir, "steamcmd.zip")
	if err := downloadFile(steamcmdZipURL, zipPath); err != nil {
		return err
	}
	defer os.Remove(zipPath)
	if err := extractZip(zipPath, dir); err != nil {
		return err
	}
	exe := filepath.Join(dir, steamcmdExeName())
	if st, err := os.Stat(exe); err != nil || st.IsDir() {
		return fmt.Errorf("в скачанном архиве нет %s", steamcmdExeName())
	}
	return nil
}

func downloadFile(url, dest string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("не удалось скачать: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	total := resp.ContentLength
	var written int64
	buf := make([]byte, 64*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			if total > 0 {
				fmt.Printf("\r  %s %d%%\x1b[K", ui.Dim("загрузка steamcmd"), written*100/total)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	fmt.Print("\r\x1b[K")
	ui.OK("steamcmd скачан (%.1f МБ)", float64(written)/1024/1024)
	return nil
}

func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		target := filepath.Join(destDir, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("недопустимый путь в архиве: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		w, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(w, rc)
		rc.Close()
		w.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Command) Bootstrap(ctx context.Context) error {
	ui.Note("инициализация steamcmd (первый запуск может занять время)...")
	err := c.run(ctx, []string{"+login", "anonymous", "+quit"}, nil)
	fmt.Print("\r\x1b[K")
	return err
}

func (c *Command) run(ctx context.Context, args []string, onLine func(string)) error {
	cmd := exec.CommandContext(ctx, c.Exe, args...)
	cmd.Dir = c.Dir
	lw := &lineWriter{onLine: onLine}
	cmd.Stdout = lw
	cmd.Stderr = lw
	err := cmd.Run()
	lw.Flush()
	return err
}

var (
	reDownloaded = regexp.MustCompile(`Success\.\s+Downloaded item (\d+)`)
	reFailed     = regexp.MustCompile(`ERROR!\s+Download item (\d+) failed`)
)

func (c *Command) DownloadAll(ctx context.Context, ids []uint64, destDir string, p *ui.Progress, verbose bool, batchSize int, labelPrefix string) ([]uint64, error) {
	if len(ids) == 0 {
		return nil, ctx.Err()
	}
	batchSize = clampBatch(batchSize)
	contentDir := c.contentDir()

	var (
		mu     sync.Mutex
		status = make(map[uint64]int, len(ids))
	)

	markOK := func(id uint64) {
		mu.Lock()
		if status[id] != 0 {
			mu.Unlock()
			return
		}
		status[id] = 1
		mu.Unlock()

		if err := moveDir(filepath.Join(contentDir, strconv.FormatUint(id, 10)), filepath.Join(destDir, strconv.FormatUint(id, 10))); err != nil {
			mu.Lock()
			status[id] = 2
			mu.Unlock()
			if verbose {
				ui.Warn("не удалось переместить скин %d: %v", id, err)
			}
			p.AddFailed()
			return
		}
		p.Add(1)
	}
	markFail := func(id uint64) {
		mu.Lock()
		if status[id] != 0 {
			mu.Unlock()
			return
		}
		status[id] = 2
		mu.Unlock()
		p.AddFailed()
	}

	for start := 0; start < len(ids); start += batchSize {
		if ctx.Err() != nil {
			break
		}
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]

		args := []string{"+login", "anonymous"}
		for _, id := range batch {
			args = append(args, "+workshop_download_item", rustAppID, strconv.FormatUint(id, 10))
		}
		args = append(args, "+quit")

		onLine := func(line string) {
			trimmed := strings.TrimSpace(line)
			if verbose && trimmed != "" {
				fmt.Printf("\r\x1b[K  %s\n", ui.Dim(trimmed))
			}
			if m := reDownloaded.FindStringSubmatch(trimmed); m != nil {
				if id, err := strconv.ParseUint(m[1], 10, 64); err == nil {
					markOK(id)
				}
			} else if m := reFailed.FindStringSubmatch(trimmed); m != nil {
				if id, err := strconv.ParseUint(m[1], 10, 64); err == nil {
					markFail(id)
				}
			}
		}

		p.SetLabel(fmt.Sprintf("%ssteamcmd, пакет %d-%d", labelPrefix, start+1, end))
		if err := c.run(ctx, args, onLine); err != nil && ctx.Err() == nil && verbose {
			ui.Warn("ошибка пакета steamcmd: %v", err)
		}

		for _, id := range batch {
			mu.Lock()
			st := status[id]
			mu.Unlock()
			if st != 0 {
				continue
			}
			if dirHasContent(filepath.Join(contentDir, strconv.FormatUint(id, 10))) {
				markOK(id)
			} else {
				markFail(id)
			}
		}
	}

	var failed []uint64
	for _, id := range ids {
		mu.Lock()
		st := status[id]
		mu.Unlock()
		if st != 1 {
			failed = append(failed, id)
		}
	}
	return failed, ctx.Err()
}

func (c *Command) DownloadParallel(ctx context.Context, baseDir string, ids []uint64, destDir string, p *ui.Progress, verbose bool, batchSize, jobs int) ([]uint64, error) {
	if jobs < 2 {
		return c.DownloadAll(ctx, ids, destDir, p, verbose, batchSize, "")
	}
	if jobs > len(ids) {
		jobs = len(ids)
	}
	const maxJobs = 16
	if jobs > maxJobs {
		jobs = maxJobs
	}

	workerRoot := filepath.Join(baseDir, "steamcmd-workers")
	chunks := make([][]uint64, jobs)
	for i, id := range ids {
		chunks[i%jobs] = append(chunks[i%jobs], id)
	}

	failCh := make(chan []uint64, jobs)
	var wg sync.WaitGroup
	for i := 0; i < jobs; i++ {
		if len(chunks[i]) == 0 {
			continue
		}
		wg.Add(1)
		go func(idx int, chunk []uint64) {
			defer wg.Done()
			workerDir := filepath.Join(workerRoot, fmt.Sprintf("w%d", idx+1))
			exe := filepath.Join(workerDir, steamcmdExeName())
			if st, err := os.Stat(exe); err != nil || st.IsDir() {
				if err := cloneSteamcmd(c.Dir, workerDir); err != nil {
					if verbose {
						ui.Warn("не удалось подготовить steamcmd для воркера %d: %v", idx+1, err)
					}
					for range chunk {
						p.AddFailed()
					}
					failCh <- chunk
					return
				}
			}
			worker := &Command{Exe: exe, Dir: workerDir}
			failed, _ := worker.DownloadAll(ctx, chunk, destDir, p, verbose, batchSize, fmt.Sprintf("[%d] ", idx+1))
			failCh <- failed
		}(i, chunks[i])
	}
	wg.Wait()
	close(failCh)

	var failed []uint64
	for f := range failCh {
		failed = append(failed, f...)
	}
	return failed, ctx.Err()
}

func clampBatch(n int) int {
	if n <= 0 {
		return 8
	}
	const maxItems = 600
	if n > maxItems {
		return maxItems
	}
	return n
}

func cloneSteamcmd(srcDir, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	skip := map[string]bool{"steamapps": true, "logs": true, "userdata": true, "appcache": true}
	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		top := strings.SplitN(rel, string(os.PathSeparator), 2)[0]
		if skip[top] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dstDir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func dirHasContent(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}

func moveDir(src, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyDir(src, dst); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

type lineWriter struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	onLine func(string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	for {
		data := w.buf.Bytes()
		i := bytes.IndexAny(data, "\n\r")
		if i < 0 {
			break
		}
		line := string(data[:i])
		w.buf.Next(i + 1)
		w.emit(line)
	}
	return len(p), nil
}

func (w *lineWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() > 0 {
		w.emit(w.buf.String())
		w.buf.Reset()
	}
}

func (w *lineWriter) emit(line string) {
	if w.onLine != nil {
		w.onLine(line)
	}
}
