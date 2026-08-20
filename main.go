//go:build windows

package main

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"golang.org/x/sys/windows"

	"youtube2mp3/internal/downloader"
)

//go:embed tools/* 춘식.jpg
var embeddedTools embed.FS

var version = "dev"

var shellExecuteW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteW")

type desktopApp struct {
	window       *walk.MainWindow
	urlEdit      *walk.LineEdit
	outputEdit   *walk.LineEdit
	qualityCombo *walk.ComboBox
	progress     *walk.ProgressBar
	statusLabel  *walk.Label
	startButton  *walk.PushButton
	cancelButton *walk.PushButton
	openButton   *walk.PushButton

	tools  downloader.Tools
	mu     sync.Mutex
	busy   bool
	cancel context.CancelFunc
}

var qualityValues = []string{"0", "256K", "192K", "128K"}

func main() {
	walk.App().SetOrganizationName("YouTube2MP3")
	walk.App().SetProductName("YouTube MP3 저장기")

	tools, err := downloader.ExtractTools(embeddedTools)
	if len(os.Args) > 1 && os.Args[1] == "--self-test" {
		if err != nil || selfTest(tools) != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	if err != nil {
		walk.MsgBox(nil, "실행 준비 실패", err.Error(), walk.MsgBoxIconError)
		return
	}

	app := &desktopApp{tools: tools}
	if err := app.run(); err != nil {
		walk.MsgBox(nil, "프로그램 오류", err.Error(), walk.MsgBoxIconError)
	}
}

func selfTest(tools downloader.Tools) error {
	checks := []struct {
		path string
		args []string
	}{
		{tools.YTDLP, []string{"--version"}},
		{tools.FFmpeg, []string{"-version"}},
		{tools.Deno, []string{"--version"}},
	}
	for _, check := range checks {
		cmd := exec.Command(check.path, check.args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	symbolImage, err := loadSymbolImage()
	if err != nil {
		return err
	}
	symbolImage.Dispose()
	return nil
}

func (a *desktopApp) run() error {
	defaultOutput := defaultDownloadDir()
	symbolImage, err := loadSymbolImage()
	if err != nil {
		return fmt.Errorf("상징 이미지를 불러올 수 없습니다: %w", err)
	}
	defer symbolImage.Dispose()

	windowDefinition := MainWindow{
		AssignTo: &a.window,
		Title:    fmt.Sprintf("YouTube MP3 저장기  %s", displayVersion()),
		Size:     Size{Width: 700, Height: 490},
		MinSize:  Size{Width: 610, Height: 450},
		Layout:   VBox{Spacing: 10},
		Children: []Widget{
			Composite{
				Layout: HBox{Spacing: 12},
				Children: []Widget{
					Composite{
						StretchFactor: 1,
						Layout:        VBox{Spacing: 5},
						Children: []Widget{
							Label{
								Text: "유튜브 영상의 음원을 MP3 파일로 저장합니다.",
								Font: Font{PointSize: 13, Bold: true},
							},
							Label{
								Text: "본인이 소유하거나 다운로드 허가를 받은 콘텐츠에만 사용하세요.",
							},
							VSpacer{},
						},
					},
					ImageView{
						Image:       symbolImage,
						Mode:        ImageViewModeZoom,
						Margin:      2,
						MinSize:     Size{Width: 112, Height: 112},
						MaxSize:     Size{Width: 112, Height: 112},
						ToolTipText: "YouTube MP3 저장기 춘식",
					},
				},
			},
			Label{Text: "유튜브 영상 주소"},
			Composite{
				Layout: HBox{Spacing: 8},
				Children: []Widget{
					LineEdit{
						AssignTo:  &a.urlEdit,
						CueBanner: "https://www.youtube.com/watch?v=...",
					},
					PushButton{Text: "붙여넣기", OnClicked: a.pasteURL},
				},
			},
			Label{Text: "저장 폴더"},
			Composite{
				Layout: HBox{Spacing: 8},
				Children: []Widget{
					LineEdit{AssignTo: &a.outputEdit, Text: defaultOutput},
					PushButton{Text: "찾아보기...", OnClicked: a.browseOutput},
				},
			},
			Composite{
				Layout: HBox{Spacing: 8},
				Children: []Widget{
					Label{Text: "MP3 음질"},
					ComboBox{
						AssignTo:     &a.qualityCombo,
						Model:        []string{"최고 음질 (VBR)", "256 kbps", "192 kbps", "128 kbps"},
						CurrentIndex: 0,
					},
					HSpacer{},
				},
			},
			ProgressBar{AssignTo: &a.progress, MinValue: 0, MaxValue: 100, Value: 0},
			Label{AssignTo: &a.statusLabel, Text: "주소를 입력하고 저장 시작을 눌러 주세요."},
			VSpacer{},
			Composite{
				Layout: HBox{Spacing: 8},
				Children: []Widget{
					PushButton{AssignTo: &a.startButton, Text: "MP3 저장 시작", OnClicked: a.startDownload},
					PushButton{AssignTo: &a.cancelButton, Text: "취소", Enabled: false, OnClicked: a.cancelDownload},
					HSpacer{},
					PushButton{AssignTo: &a.openButton, Text: "저장 폴더 열기", OnClicked: a.openOutput},
				},
			},
		},
	}
	if err := windowDefinition.Create(); err != nil {
		return err
	}
	a.window.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		a.cancelCurrent()
	})
	a.window.Run()
	return nil
}

func loadSymbolImage() (*walk.Bitmap, error) {
	data, err := embeddedTools.ReadFile("춘식.jpg")
	if err != nil {
		return nil, err
	}
	source, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	thumbnail := downsampleImage(source, 144, 144)
	return walk.NewBitmapFromImageForDPI(thumbnail, 96)
}

// downsampleImage uses a small area-sampling grid per destination pixel. It
// preserves the thin pen strokes in the source better than nearest-neighbor
// scaling while keeping startup work small.
func downsampleImage(source image.Image, width, height int) *image.RGBA {
	destination := image.NewRGBA(image.Rect(0, 0, width, height))
	bounds := source.Bounds()
	const samples = 6
	const sampleCount = samples * samples

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var red, green, blue, alpha uint64
			for sampleY := 0; sampleY < samples; sampleY++ {
				sourceY := bounds.Min.Y + ((y*samples+sampleY)*bounds.Dy())/(height*samples)
				for sampleX := 0; sampleX < samples; sampleX++ {
					sourceX := bounds.Min.X + ((x*samples+sampleX)*bounds.Dx())/(width*samples)
					r, g, b, a := source.At(sourceX, sourceY).RGBA()
					red += uint64(r)
					green += uint64(g)
					blue += uint64(b)
					alpha += uint64(a)
				}
			}
			destination.SetRGBA(x, y, color.RGBA{
				R: uint8((red / sampleCount) >> 8),
				G: uint8((green / sampleCount) >> 8),
				B: uint8((blue / sampleCount) >> 8),
				A: uint8((alpha / sampleCount) >> 8),
			})
		}
	}
	return destination
}

func (a *desktopApp) pasteURL() {
	text, err := walk.Clipboard().Text()
	if err != nil {
		a.showError("클립보드의 텍스트를 읽을 수 없습니다.")
		return
	}
	a.urlEdit.SetText(strings.TrimSpace(text))
	a.urlEdit.SetFocus()
}

func (a *desktopApp) browseOutput() {
	dialog := new(walk.FileDialog)
	dialog.Title = "MP3 저장 폴더 선택"
	dialog.FilePath = strings.TrimSpace(a.outputEdit.Text())
	ok, err := dialog.ShowBrowseFolder(a.window)
	if err != nil {
		a.showError("폴더 선택 창을 열 수 없습니다.")
		return
	}
	if ok {
		a.outputEdit.SetText(dialog.FilePath)
	}
}

func (a *desktopApp) startDownload() {
	videoURL := strings.TrimSpace(a.urlEdit.Text())
	if err := downloader.ValidateYouTubeURL(videoURL); err != nil {
		a.showError(err.Error())
		a.urlEdit.SetFocus()
		return
	}
	outputDir := strings.TrimSpace(a.outputEdit.Text())
	if outputDir == "" {
		a.showError("저장 폴더를 선택해 주세요.")
		return
	}
	qualityIndex := a.qualityCombo.CurrentIndex()
	if qualityIndex < 0 || qualityIndex >= len(qualityValues) {
		qualityIndex = 0
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	if a.busy {
		a.mu.Unlock()
		cancel()
		return
	}
	a.busy = true
	a.cancel = cancel
	a.mu.Unlock()

	a.setBusyUI(true)
	a.progress.SetValue(0)
	a.statusLabel.SetText("영상 정보를 확인하는 중...")

	req := downloader.Request{URL: videoURL, OutputDir: outputDir, Quality: qualityValues[qualityIndex]}
	go func() {
		defer cancel()
		err := downloader.Download(ctx, a.tools, req, func(event downloader.Event) {
			a.window.Synchronize(func() {
				if event.Percent >= 0 {
					a.progress.SetValue(int(event.Percent))
				}
				if event.Status != "" {
					a.statusLabel.SetText(event.Status)
				}
			})
		})
		a.window.Synchronize(func() { a.downloadFinished(err) })
	}()
}

func (a *desktopApp) downloadFinished(err error) {
	a.mu.Lock()
	a.busy = false
	a.cancel = nil
	a.mu.Unlock()
	a.setBusyUI(false)

	switch {
	case err == nil:
		a.progress.SetValue(100)
		a.statusLabel.SetText("완료되었습니다. 선택한 폴더에 MP3 파일을 저장했습니다.")
		walk.MsgBox(a.window, "저장 완료", "MP3 파일을 저장했습니다.", walk.MsgBoxIconInformation)
	case errors.Is(err, downloader.ErrCancelled):
		a.progress.SetValue(0)
		a.statusLabel.SetText("다운로드를 취소했습니다.")
	default:
		a.progress.SetValue(0)
		a.statusLabel.SetText("저장하지 못했습니다. 오류 내용을 확인해 주세요.")
		a.showError(err.Error())
	}
}

func (a *desktopApp) cancelDownload() {
	a.statusLabel.SetText("취소하는 중...")
	a.cancelCurrent()
}

func (a *desktopApp) cancelCurrent() {
	a.mu.Lock()
	cancel := a.cancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *desktopApp) setBusyUI(busy bool) {
	a.startButton.SetEnabled(!busy)
	a.cancelButton.SetEnabled(busy)
	a.urlEdit.SetReadOnly(busy)
	a.outputEdit.SetReadOnly(busy)
	a.qualityCombo.SetEnabled(!busy)
}

func (a *desktopApp) openOutput() {
	path := strings.Trim(strings.TrimSpace(a.outputEdit.Text()), "\"")
	if path == "" {
		return
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		a.showError("저장 폴더를 열 수 없습니다: " + err.Error())
		return
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		a.showError("저장 폴더 경로를 확인할 수 없습니다: " + err.Error())
		return
	}
	if err := openFolderWithShell(uintptr(a.window.Handle()), absolutePath); err != nil {
		a.showError("저장 폴더를 열 수 없습니다: " + err.Error())
	}
}

// openFolderWithShell asks Windows to open a folder through the system shell.
// This is more reliable than launching explorer.exe directly because Explorer
// commonly reuses an existing process and may silently ignore process arguments.
func openFolderWithShell(owner uintptr, path string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	result, _, callErr := shellExecuteW.Call(
		owner,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(target)),
		0,
		0,
		1, // SW_SHOWNORMAL
	)
	if result <= 32 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return fmt.Errorf("Windows 셸 오류 %d: %w", result, callErr)
		}
		return fmt.Errorf("Windows 셸 오류 코드 %d", result)
	}
	return nil
}

func (a *desktopApp) showError(message string) {
	walk.MsgBox(a.window, "확인해 주세요", message, walk.MsgBoxIconError)
}

func defaultDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	downloads := filepath.Join(home, "Downloads")
	if info, err := os.Stat(downloads); err == nil && info.IsDir() {
		return downloads
	}
	return home
}

func displayVersion() string {
	if version == "" || version == "dev" {
		return ""
	}
	return "v" + version
}
