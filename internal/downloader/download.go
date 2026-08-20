package downloader

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	progressPattern = regexp.MustCompile(`\[download\]\s+([0-9]+(?:\.[0-9]+)?)%`)
	errorPattern    = regexp.MustCompile(`(?i)^ERROR:\s*`)
)

var ErrCancelled = errors.New("다운로드가 취소되었습니다")

type Request struct {
	URL       string
	OutputDir string
	Quality   string
}

type Event struct {
	Percent   float64
	Status    string
	FinalPath string
}

// Download invokes the embedded yt-dlp and FFmpeg tools and streams progress.
func Download(ctx context.Context, tools Tools, req Request, emit func(Event)) error {
	if err := ValidateYouTubeURL(req.URL); err != nil {
		return err
	}
	if strings.TrimSpace(req.OutputDir) == "" {
		return fmt.Errorf("저장 폴더를 선택해 주세요")
	}
	if err := os.MkdirAll(req.OutputDir, 0o755); err != nil {
		return fmt.Errorf("저장 폴더를 만들 수 없습니다: %w", err)
	}
	quality := req.Quality
	if quality == "" {
		quality = "0"
	}

	args := []string{
		"--ignore-config",
		"--no-playlist",
		"--newline",
		"--progress",
		"--no-colors",
		"--encoding", "utf-8",
		"--windows-filenames",
		"--trim-filenames", "180",
		"--no-overwrites",
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", quality,
		"--embed-metadata",
		"--ffmpeg-location", tools.FFmpeg,
		"--js-runtimes", "deno:" + tools.Deno,
		"--paths", req.OutputDir,
		"--output", "%(title)s [%(id)s].%(ext)s",
		"--print", "after_move:__FINAL__:%(filepath)s",
		"--", strings.TrimSpace(req.URL),
	}

	cmd := exec.CommandContext(ctx, tools.YTDLP, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x00000200}
	cmd.WaitDelay = 3 * time.Second
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		// yt-dlp may have FFmpeg or Deno children. Kill only this process tree so
		// clicking Cancel does not leave a converter running in the background.
		killer := exec.Command("taskkill.exe", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
		killer.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := killer.Run(); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("다운로더 출력 연결 실패: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("다운로더 오류 출력 연결 실패: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("다운로더를 시작할 수 없습니다: %w", err)
	}

	lines := make(chan string, 64)
	var readers sync.WaitGroup
	readers.Add(2)
	read := func(scanner *bufio.Scanner) {
		defer readers.Done()
		for scanner.Scan() {
			lines <- strings.TrimSpace(scanner.Text())
		}
	}
	go read(bufio.NewScanner(stdout))
	go read(bufio.NewScanner(stderr))
	go func() {
		readers.Wait()
		close(lines)
	}()

	lastError := ""
	for line := range lines {
		if line == "" {
			continue
		}
		if match := progressPattern.FindStringSubmatch(line); len(match) == 2 {
			if percent, parseErr := strconv.ParseFloat(match[1], 64); parseErr == nil {
				emit(Event{Percent: percent, Status: fmt.Sprintf("다운로드 중... %.1f%%", percent)})
			}
			continue
		}
		if strings.HasPrefix(line, "__FINAL__:") {
			finalPath := strings.TrimSpace(strings.TrimPrefix(line, "__FINAL__:"))
			emit(Event{Percent: 100, Status: "MP3 저장 완료", FinalPath: finalPath})
			continue
		}
		if strings.HasPrefix(line, "[ExtractAudio]") {
			emit(Event{Percent: 100, Status: "MP3로 변환 중..."})
			continue
		}
		if strings.HasPrefix(line, "[Metadata]") {
			emit(Event{Percent: 100, Status: "메타데이터 기록 중..."})
			continue
		}
		if errorPattern.MatchString(line) {
			lastError = errorPattern.ReplaceAllString(line, "")
		}
	}

	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return ErrCancelled
	}
	if waitErr != nil {
		if lastError != "" {
			return fmt.Errorf("%s", friendlyError(lastError))
		}
		return fmt.Errorf("다운로드에 실패했습니다: %w", waitErr)
	}
	return nil
}

func ValidateYouTubeURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("유튜브 영상 주소를 입력해 주세요")
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return fmt.Errorf("올바른 유튜브 주소를 입력해 주세요")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("올바른 유튜브 주소를 입력해 주세요")
	}
	host := strings.ToLower(u.Hostname())
	validHost := host == "youtu.be" || host == "youtube.com" ||
		strings.HasSuffix(host, ".youtube.com") || host == "youtube-nocookie.com" ||
		strings.HasSuffix(host, ".youtube-nocookie.com")
	if !validHost {
		return fmt.Errorf("유튜브 영상 주소만 사용할 수 있습니다")
	}
	if u.Path == "" || u.Path == "/" {
		return fmt.Errorf("영상이 포함된 유튜브 주소를 입력해 주세요")
	}
	return nil
}

func friendlyError(message string) string {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "private video"):
		return "비공개 영상은 다운로드할 수 없습니다"
	case strings.Contains(lower, "video unavailable") || strings.Contains(lower, "not available"):
		return "사용할 수 없거나 삭제된 영상입니다"
	case strings.Contains(lower, "sign in") || strings.Contains(lower, "login"):
		return "로그인이 필요한 영상은 이 앱에서 다운로드할 수 없습니다"
	case strings.Contains(lower, "copyright"):
		return "저작권 제한으로 이 영상을 처리할 수 없습니다"
	default:
		return message
	}
}
