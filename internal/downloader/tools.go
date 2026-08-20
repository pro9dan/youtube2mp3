package downloader

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// Tools are the helper executables extracted from the single-file application.
type Tools struct {
	YTDLP  string
	FFmpeg string
	Deno   string
}

// ExtractTools extracts embedded helper binaries to a versioned local cache.
// Reusing a hash-based directory avoids rewriting large files on every launch.
func ExtractTools(toolFS embed.FS) (Tools, error) {
	ytdlp, err := toolFS.ReadFile("tools/yt-dlp.exe")
	if err != nil {
		return Tools{}, fmt.Errorf("내장 yt-dlp를 찾을 수 없습니다. build.ps1로 다시 빌드해 주세요: %w", err)
	}
	ffmpeg, err := toolFS.ReadFile("tools/ffmpeg.exe")
	if err != nil {
		return Tools{}, fmt.Errorf("내장 FFmpeg를 찾을 수 없습니다. build.ps1로 다시 빌드해 주세요: %w", err)
	}
	deno, err := toolFS.ReadFile("tools/deno.exe")
	if err != nil {
		return Tools{}, fmt.Errorf("내장 Deno를 찾을 수 없습니다. build.ps1로 다시 빌드해 주세요: %w", err)
	}
	if !isWindowsExecutable(ytdlp) || !isWindowsExecutable(ffmpeg) || !isWindowsExecutable(deno) {
		return Tools{}, fmt.Errorf("내장 도구 파일이 올바른 Windows 실행 파일이 아닙니다")
	}

	hash := sha256.New()
	_, _ = hash.Write(ytdlp)
	_, _ = hash.Write(ffmpeg)
	_, _ = hash.Write(deno)
	cacheID := hex.EncodeToString(hash.Sum(nil))[:16]

	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "YouTube2MP3", cacheID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Tools{}, fmt.Errorf("내장 도구 폴더를 만들 수 없습니다: %w", err)
	}

	tools := Tools{
		YTDLP:  filepath.Join(dir, "yt-dlp.exe"),
		FFmpeg: filepath.Join(dir, "ffmpeg.exe"),
		Deno:   filepath.Join(dir, "deno.exe"),
	}
	if err := writeIfNeeded(tools.YTDLP, ytdlp); err != nil {
		return Tools{}, fmt.Errorf("yt-dlp 준비 실패: %w", err)
	}
	if err := writeIfNeeded(tools.FFmpeg, ffmpeg); err != nil {
		return Tools{}, fmt.Errorf("FFmpeg 준비 실패: %w", err)
	}
	if err := writeIfNeeded(tools.Deno, deno); err != nil {
		return Tools{}, fmt.Errorf("Deno 준비 실패: %w", err)
	}

	return tools, nil
}

func isWindowsExecutable(data []byte) bool {
	return len(data) >= 2 && data[0] == 'M' && data[1] == 'Z'
}

func writeIfNeeded(path string, data []byte) error {
	if current, err := os.ReadFile(path); err == nil {
		currentHash := sha256.Sum256(current)
		wantedHash := sha256.Sum256(data)
		if currentHash == wantedHash {
			return nil
		}
	}

	temp, err := os.CreateTemp(filepath.Dir(path), ".extract-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	_ = os.Remove(path)
	return os.Rename(tempPath, path)
}
