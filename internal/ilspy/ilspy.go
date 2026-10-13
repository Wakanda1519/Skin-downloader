package ilspy

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/Wakanda1519/skindl/internal/ui"
)

type Tool struct {
	Exe string
}

var versionRe = regexp.MustCompile(`\d+\.\d+\.\d+(?:\.\d+)?`)

func exeName() string {
	if runtime.GOOS == "windows" {
		return "ilspycmd.exe"
	}
	return "ilspycmd"
}

func Find(baseDir, override string) (*Tool, error) {
	if override != "" {
		abs, err := filepath.Abs(override)
		if err != nil {
			return nil, err
		}
		if st, err := os.Stat(abs); err != nil || st.IsDir() {
			return nil, fmt.Errorf("ilspycmd не найден по пути %s", abs)
		}
		return &Tool{Exe: abs}, nil
	}

	if p := locate(baseDir); p != "" {
		return &Tool{Exe: p}, nil
	}

	if _, err := exec.LookPath("dotnet"); err != nil {
		return nil, fmt.Errorf("ilspycmd не найден, а .NET SDK (dotnet) недоступен; установите .NET SDK: https://dotnet.microsoft.com/download")
	}

	ui.Warn("ilspycmd не найден, устанавливаем через 'dotnet tool install -g ilspycmd'...")
	install := exec.Command("dotnet", "tool", "install", "-g", "ilspycmd")
	install.Env = quietDotnetEnv()
	var out strings.Builder
	install.Stdout = &out
	install.Stderr = &out
	if err := install.Run(); err != nil {
		msg := strings.TrimSpace(out.String())
		if msg != "" {
			return nil, fmt.Errorf("не удалось установить ilspycmd: %w\n%s", err, msg)
		}
		return nil, fmt.Errorf("не удалось установить ilspycmd: %w", err)
	}
	if v := versionRe.FindString(out.String()); v != "" {
		ui.OK("средство \"ilspycmd\" (версии \"%s\") успешно установлено", v)
	} else {
		ui.OK("средство \"ilspycmd\" успешно установлено")
	}

	if p := locate(baseDir); p != "" {
		return &Tool{Exe: p}, nil
	}
	return nil, fmt.Errorf("ilspycmd установлен, но не найден; добавьте %s в PATH и повторите", toolsDir())
}

func locate(baseDir string) string {
	candidates := []string{filepath.Join(baseDir, exeName())}
	if dir := toolsDir(); dir != "" {
		candidates = append(candidates, filepath.Join(dir, exeName()))
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	if p, err := exec.LookPath(exeName()); err == nil {
		return p
	}
	return ""
}

func Remove(baseDir string) error {
	local := filepath.Join(baseDir, exeName())
	if st, err := os.Stat(local); err == nil && !st.IsDir() {
		if err := os.Remove(local); err != nil {
			return err
		}
	}
	if _, err := exec.LookPath("dotnet"); err == nil {
		cmd := exec.Command("dotnet", "tool", "uninstall", "-g", "ilspycmd")
		cmd.Env = quietDotnetEnv()
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		_ = cmd.Run()
	}
	return nil
}

func quietDotnetEnv() []string {
	return append(os.Environ(),
		"DOTNET_NOLOGO=1",
		"DOTNET_CLI_TELEMETRY_OPTOUT=1",
		"DOTNET_SKIP_FIRST_TIME_EXPERIENCE=1",
	)
}

func toolsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".dotnet", "tools")
}

func (t *Tool) Decompile(ctx context.Context, dllPath, outDir string, verbose bool) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, t.Exe, "--ignore-decompilation-errors", "-p", "-o", outDir, dllPath)
	var out strings.Builder
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = &out
		cmd.Stderr = &out
	}

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(out.String())
		if msg != "" {
			return fmt.Errorf("ilspycmd завершился с ошибкой: %w\n%s", err, msg)
		}
		return fmt.Errorf("ilspycmd завершился с ошибкой: %w", err)
	}
	return nil
}
