package ytdlp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func DownloadMp3(ctx context.Context, link, tempDir, targetDir string) (name string, path string, err error) {
	tempDir, terr := os.MkdirTemp(tempDir, "yt-dlp")
	if terr != nil {
		return "", "", fmt.Errorf("creating temporary directory: %w", terr)
	}

	defer func() {
		errDefer := os.RemoveAll(tempDir)
		if err == nil && errDefer != nil {
			err = fmt.Errorf("removing temporary directory: %w", errDefer)
		}
	}()

	tempDir, terr = filepath.Abs(tempDir)
	if terr != nil {
		return "", "", fmt.Errorf("getting absolute path for temporary directory: %w", terr)
	}

	cmd := exec.CommandContext(ctx,
		"yt-dlp", "--quiet",
		"--paths", "temp:"+tempDir,
		"--paths", "home:"+targetDir,
		"-f", "bestaudio",
		"--extract-audio",
		"--audio-format", "mp3",
		"--print", "before_dl:%(title)U",
		"--print", "after_move:filepath",
		"--", link,
	)

	stdout, stderr := new(strings.Builder), new(strings.Builder)
	cmd.Stdout, cmd.Stderr = stdout, stderr

	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("executing yt-dlp: %w: %s", err, stderr.String())
	}

	stdoutSl := strings.Split(stdout.String(), "\n")
	if len(stdoutSl) != 3 { // 3rd is trailing newline
		return "", "", fmt.Errorf("splitting stdout: len(stdout) expected 3, got %d", len(stdoutSl))
	}

	name, path = stdoutSl[0], stdoutSl[1]
	return
}
