package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Wakanda1519/skindl/internal/approved"
	"github.com/Wakanda1519/skindl/internal/steamcmd"
	"github.com/Wakanda1519/skindl/internal/ui"
)

const (
	sourceDirName   = "source"
	approvedRelPath = "Rust.Workshop/Approved.cs"
	workshopDirName = "workshop"
	skinsJSONName   = "Skins.json"
	failedFileName  = "failed.txt"
	totalSteps      = 3
)

func main() {
	dir := flag.String("dir", "", "рабочая папка (по умолчанию текущая)")
	steamcmdPath := flag.String("steamcmd", "", "путь к steamcmd.exe")
	limit := flag.Int("limit", 0, "скачать только первые N скинов (0 = все)")
	noDownload := flag.Bool("no-download", false, "только разобрать Approved.cs и создать skins.json")
	yes := flag.Bool("yes", false, "не ждать Enter, считать исходники готовыми")
	verbose := flag.Bool("verbose", false, "показывать сырой вывод steamcmd")
	batch := flag.Int("batch", 8, "сколько скинов скачивать за один вызов steamcmd (меньше — прогресс обновляется чаще)")
	jobs := flag.Int("jobs", 1, "число параллельных воркеров steamcmd (ускоряет скачивание)")
	status := flag.Bool("status", false, "показать, сколько уже скачано, и выйти")
	retries := flag.Int("retries", 3, "сколько раз повторять неудачные скины (0 = без повторов)")
	flag.Parse()

	ui.Init()
	ui.Banner()

	baseDir, err := resolveBaseDir(*dir)
	if err != nil {
		fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ui.Step(1, totalSteps, "Подготовка папки source")
	sourceDir := filepath.Join(baseDir, sourceDirName)
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		fatal(err)
	}
	ui.OK("готово: %s", sourceDir)
	approvedPath := waitForSources(sourceDir, *yes)
	fmt.Println()

	ui.Step(2, totalSteps, "Анализ Approved.cs")
	skins, err := approved.Parse(approvedPath)
	if err != nil {
		fatal(fmt.Errorf("не удалось прочитать %s: %w", approvedPath, err))
	}
	if len(skins) == 0 {
		fatal(fmt.Errorf("в %s не найдено ни одного скина (неожиданный формат?)", approvedPath))
	}
	groups := approved.Group(skins)
	ids := approved.UniqueIDs(skins)

	jsonPath := filepath.Join(baseDir, skinsJSONName)
	if err := approved.WriteJSON(jsonPath, groups); err != nil {
		fatal(err)
	}
	ui.OK("разобрано скинов: %d", len(skins))
	ui.OK("создан %s  (предметов: %d)", jsonPath, len(groups))
	fmt.Println()

	if *noDownload {
		printSummary(baseDir, len(skins), len(groups), len(ids), 0, 0)
		return
	}

	workshopDir := filepath.Join(baseDir, workshopDirName)
	if err := os.MkdirAll(workshopDir, 0o755); err != nil {
		fatal(err)
	}

	if *status {
		_, already := steamcmd.FilterPending(workshopDir, ids)
		printStatus(baseDir, len(ids), already)
		return
	}

	ui.Step(3, totalSteps, "Скачивание скинов из Workshop")
	if *limit > 0 && *limit < len(ids) {
		ui.Note("включён лимит: качаем первые %d из %d скинов", *limit, len(ids))
		ids = ids[:*limit]
	}

	pending, already := steamcmd.FilterPending(workshopDir, ids)
	if len(pending) == 0 {
		ui.OK("все скины уже скачаны (%d)", already)
		printSummary(baseDir, len(skins), len(groups), len(ids), len(ids), 0)
		return
	}
	if already > 0 {
		ui.OK("уже скачано: %d, к загрузке: %d", already, len(pending))
	}

	sc, err := steamcmd.Find(baseDir, *steamcmdPath)
	if err != nil {
		fatal(err)
	}
	ui.OK("steamcmd: %s", sc.Exe)
	if sc.Fresh {
		_ = sc.Bootstrap(ctx)
	}

	if *jobs > 1 {
		ui.Note("параллельный режим: воркеров steamcmd — %d", *jobs)
	}
	failed := runDownloads(ctx, sc, baseDir, workshopDir, pending, already, *verbose, *batch, *jobs, *retries)

	if len(failed) > 0 {
		failedPath := filepath.Join(baseDir, failedFileName)
		if err := writeFailedList(failedPath, failed); err == nil {
			ui.Warn("не скачалось скинов: %d; их ID записаны в %s", len(failed), failedPath)
		}
	}

	if ctx.Err() != nil {
		ui.Warn("остановлено пользователем")
	}

	pruned := false
	if ctx.Err() == nil {
		reader := bufio.NewReader(os.Stdin)
		missing, _ := steamcmd.FilterPending(workshopDir, ids)
		for len(missing) > 0 && ctx.Err() == nil {
			switch askFinalChoice(reader, len(missing)) {
			case 1:
				_ = runDownloads(ctx, sc, baseDir, workshopDir, missing, 0, *verbose, *batch, *jobs, *retries)
				missing, _ = steamcmd.FilterPending(workshopDir, ids)
			case 2:
				remove := make(map[uint64]struct{}, len(missing))
				for _, id := range missing {
					remove[id] = struct{}{}
				}
				groups = approved.RemoveIDs(groups, remove)
				if err := approved.WriteJSON(jsonPath, groups); err != nil {
					fatal(err)
				}
				ui.OK("обновлён %s: убрано id — %d", jsonPath, len(missing))
				pruned = true
				missing = nil
			default:
				missing = nil
			}
		}
	}

	finalMissing, finalPresent := steamcmd.FilterPending(workshopDir, ids)
	if len(finalMissing) > 0 && !pruned {
		ui.Warn("по факту на диске не хватает скинов: %d (см. %s)", len(finalMissing), failedFileName)
	}
	failedCount := len(finalMissing)
	if pruned {
		failedCount = 0
	}
	printSummary(baseDir, len(skins), len(groups), len(ids), finalPresent, failedCount)
}

func runDownloads(ctx context.Context, sc *steamcmd.Command, baseDir, workshopDir string, ids []uint64, already int, verbose bool, batch, jobs, retries int) []uint64 {
	progress := ui.NewProgress(len(ids) + already)
	progress.Start()
	if already > 0 {
		progress.SetLabel(fmt.Sprintf("уже скачано: %d", already))
		progress.Add(already)
	}

	run := func(list []uint64, p *ui.Progress) []uint64 {
		if jobs > 1 {
			f, _ := sc.DownloadParallel(ctx, baseDir, list, workshopDir, p, verbose, batch, jobs)
			return f
		}
		f, _ := sc.DownloadAll(ctx, list, workshopDir, p, verbose, batch, "")
		return f
	}

	failed := run(ids, progress)
	progress.Finish()

	for attempt := 1; attempt <= retries && len(failed) > 0 && ctx.Err() == nil; attempt++ {
		ui.Note("повторная попытка %d/%d: скинов — %d", attempt, retries, len(failed))
		retryProgress := ui.NewProgress(len(failed))
		retryProgress.Start()
		failed = run(failed, retryProgress)
		retryProgress.Finish()
	}
	return failed
}

func askFinalChoice(reader *bufio.Reader, missing int) int {
	for {
		fmt.Println()
		fmt.Println(ui.Bold("  Итоговая проверка"))
		ui.Warn("заявлено скачанными, но нет на диске: %d", missing)
		fmt.Println("  1 - докачать недостающие")
		fmt.Println("  2 - удалить не скачанные id из Skins.json")
		fmt.Print(ui.Bold("  Выберите 1 или 2: "))
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println()
			return 0
		}
		switch strings.TrimSpace(line) {
		case "1":
			return 1
		case "2":
			return 2
		default:
			ui.Err("введите 1 или 2")
		}
	}
}

func resolveBaseDir(dir string) (string, error) {
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = cwd
	}
	return filepath.Abs(dir)
}

func waitForSources(sourceDir string, skipWait bool) string {
	expected := filepath.Join(sourceDir, filepath.FromSlash(approvedRelPath))

	if skipWait {
		if p, ok := findApproved(sourceDir); ok {
			ui.Note("используется %s", p)
			return p
		}
		fatal(fmt.Errorf("исходники не готовы: %s не найден", expected))
	}

	ui.Note("Ищем декомпилированные исходники по пути:")
	fmt.Printf("    %s\n\n", ui.Cyan(expected))
	ui.Note("Положите результат декомпиляции Rust.Workshop в папку:")
	fmt.Printf("    %s\n\n", ui.Cyan(sourceDir))

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(ui.Bold("Нажмите Enter, когда файл будет на месте... "))
		if _, err := reader.ReadString('\n'); err != nil {
			fatal(err)
		}
		fmt.Println()

		p, ok := findApproved(sourceDir)
		if ok {
			if !strings.EqualFold(p, expected) {
				ui.Note("используется %s", p)
			}
			ui.OK("найден %s", filepath.Base(p))
			return p
		}
		ui.Warn("пока не найден, ожидается: %s", expected)
	}
}

func findApproved(sourceDir string) (string, bool) {
	expected := filepath.Join(sourceDir, filepath.FromSlash(approvedRelPath))
	if st, err := os.Stat(expected); err == nil && !st.IsDir() {
		return expected, true
	}

	var matches []string
	_ = filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), "Approved.cs") {
			matches = append(matches, path)
		}
		return nil
	})
	if len(matches) == 0 {
		return "", false
	}
	sort.Slice(matches, func(i, j int) bool {
		return strings.Contains(matches[i], "Rust.Workshop") && !strings.Contains(matches[j], "Rust.Workshop")
	})
	return matches[0], true
}

func writeFailedList(path string, failed []uint64) error {
	var b strings.Builder
	for _, id := range failed {
		b.WriteString(strconv.FormatUint(id, 10))
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func printStatus(baseDir string, total, already int) {
	fmt.Println()
	fmt.Println(ui.Bold("  Состояние загрузки"))
	fmt.Printf("  всего скинов:    %d\n", total)
	fmt.Printf("  уже скачано:     %d\n", already)
	fmt.Printf("  осталось:        %d\n", total-already)
	fmt.Printf("  папка workshop:  %s\n", filepath.Join(baseDir, workshopDirName))
	fmt.Println()
}

func printSummary(baseDir string, skins, items, ids, ok, failed int) {
	fmt.Println()
	fmt.Println(ui.Bold("  Готово"))
	fmt.Printf("  скинов разобрано: %d\n", skins)
	fmt.Printf("  типов предметов:  %d\n", items)
	fmt.Printf("  уникальных ID:    %d\n", ids)
	if ids > 0 {
		fmt.Printf("  скачано:          %d\n", ok)
		if failed > 0 {
			fmt.Printf("  ошибок:           %d\n", failed)
		}
	}
	fmt.Printf("  JSON:             %s\n", filepath.Join(baseDir, skinsJSONName))
	fmt.Printf("  папка workshop:   %s\n", filepath.Join(baseDir, workshopDirName))
	fmt.Println()
}

func fatal(err error) {
	ui.Err("%v", err)
	os.Exit(1)
}
