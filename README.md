<img src="./춘식.jpg" alt="YouTube MP3 저장기 춘식 상징 이미지" width="160" align="right">

# YouTube MP3 저장기

유튜브 영상 주소를 입력하면 음원을 내려받아 MP3 파일로 변환하는 Windows 데스크톱 프로그램입니다.

`yt-dlp`, FFmpeg, Deno가 실행 파일 안에 포함되어 있어 별도 프로그램을 설치하지 않고 **EXE 파일 하나로 실행**할 수 있습니다.

![Windows](https://img.shields.io/badge/Windows-10%20%7C%2011-0078D4?logo=windows&logoColor=white)
![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)
![Version](https://img.shields.io/badge/version-1.0.2-orange)

> 본인이 소유하거나 다운로드 및 변환 권한을 받은 콘텐츠에만 사용하세요.

<br clear="right">

## 주요 기능

- 유튜브 영상 URL 입력 및 클립보드 붙여넣기
- MP3 저장 폴더 선택
- 최고 음질(VBR), 256 kbps, 192 kbps, 128 kbps 선택
- 다운로드 및 MP3 변환 진행률 표시
- 작업 취소
- 저장 폴더 바로 열기
- Windows에서 사용할 수 없는 파일명 문자와 긴 파일명 자동 처리
- 영상 제목과 ID를 사용한 중복 방지 파일명 생성
- 춘식 상징 이미지가 포함된 Windows GUI
- `yt-dlp`, FFmpeg, Deno를 내장한 단일 EXE 배포

재생목록 주소를 입력하더라도 현재 영상 하나만 처리합니다.

## 다운로드 및 실행

1. 저장소의 [Releases](https://github.com/pro9dan/youtube2mp3/releases) 페이지에서 최신 `YouTube2MP3.exe`를 내려받습니다.
2. EXE 파일을 실행합니다.
3. 유튜브 영상 주소를 붙여넣습니다.
4. 저장 폴더와 MP3 음질을 선택합니다.
5. **MP3 저장 시작**을 누릅니다.

완성된 파일은 다음 형식으로 저장됩니다.

```text
영상 제목 [YouTube 영상 ID].mp3
```

서명되지 않은 개인 빌드는 처음 실행할 때 Windows SmartScreen 경고가 나타날 수 있습니다. 파일의 출처를 확인한 후 실행하세요.

## 지원 환경

- Windows 10 또는 Windows 11
- 64비트(x64) 환경
- 인터넷 연결

비공개 영상, 삭제된 영상, 로그인 또는 연령 확인이 필요한 영상은 지원하지 않습니다.

## 직접 빌드하기

### 준비 사항

- Windows 10/11 x64
- [Go](https://go.dev/dl/) 1.22 이상
- PowerShell
- 인터넷 연결

### 빌드 명령

저장소 루트에서 다음 명령을 실행합니다.

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\build.ps1
```

빌드 스크립트가 다음 작업을 자동으로 수행합니다.

1. 공식 `yt-dlp.exe` 다운로드
2. BtbN의 LGPL FFmpeg Windows 빌드 다운로드
3. 공식 Deno Windows 런타임 다운로드
4. Go 모듈 다운로드 및 테스트
5. Windows GUI 매니페스트 생성
6. 내장 도구와 춘식 이미지를 포함한 단일 EXE 생성
7. 패키징된 EXE 자체 테스트

빌드 결과는 다음 위치에 생성됩니다.

```text
dist\YouTube2MP3.exe
```

내장 도구를 최신 버전으로 다시 내려받아 빌드하려면 다음 명령을 사용합니다.

```powershell
.\build.ps1 -RefreshTools -Version 1.0.3
```

## 테스트

```powershell
go test ./...
go vet ./...
```

`build.ps1`은 단위 테스트뿐 아니라 완성된 EXE에서 내장 `yt-dlp`, FFmpeg, Deno와 상징 이미지가 정상적으로 로드되는지도 검사합니다.

## 프로젝트 구조

```text
youtube2mp3/
├── main.go                       # Windows GUI와 애플리케이션 진입점
├── internal/downloader/
│   ├── download.go              # yt-dlp 실행 및 진행률 처리
│   ├── download_test.go         # URL 및 오류 처리 테스트
│   └── tools.go                 # 내장 도구 추출 및 무결성 확인
├── tools/README.md               # 빌드 시 내려받는 도구 안내
├── 춘식.jpg                      # 프로그램 상징 이미지
├── app.manifest                 # Windows 호환성 및 DPI 설정
├── build.ps1                    # 단일 EXE 빌드 스크립트
├── go.mod
├── go.sum
└── THIRD_PARTY_NOTICES.md       # 서드파티 라이선스 고지
```

`dist/`, `.build/`, `tools/*.exe`는 `.gitignore`에 포함됩니다. 약 230MB 크기의 완성된 EXE는 Git 저장소에 커밋하지 않고 GitHub Releases에 첨부하는 것을 권장합니다.

## 동작 방식

애플리케이션은 EXE에 포함된 `yt-dlp`, FFmpeg, Deno를 처음 실행할 때 사용자 캐시 폴더 아래의 해시 기반 디렉터리에 추출합니다. 이후 실행에서는 같은 파일을 재사용하므로 사용자는 EXE 하나만 보관하거나 전달하면 됩니다.

다운로드는 별도 작업에서 실행되어 GUI가 멈추지 않으며, 취소 시 해당 작업에서 생성된 자식 프로세스도 함께 종료합니다.

## 문제 해결

### 영상 다운로드에 실패하는 경우

유튜브 변경 사항에 맞춰 내장 도구를 갱신한 뒤 다시 빌드해 보세요.

```powershell
.\build.ps1 -RefreshTools
```

### 저장 폴더가 열리지 않는 경우

v1.0.1 이상을 사용하세요. 해당 버전부터 Windows 셸 API로 저장 폴더를 엽니다.

### 빌드 중 임시 파일 잠금 오류가 발생하는 경우

Windows 보안 프로그램이 대용량 EXE를 검사하는 동안 일시적인 파일 잠금이 발생할 수 있습니다. 빌드 스크립트는 자동으로 최대 3회 재시도합니다.

## 저작권 및 이용 책임

이 프로젝트는 개인이 소유하거나 다운로드 허가를 받은 콘텐츠를 MP3로 보관하기 위한 도구입니다. 사용자는 YouTube 이용약관, 저작권법과 거주 지역의 관련 법규를 준수할 책임이 있습니다.

서드파티 구성요소와 라이선스는 [THIRD_PARTY_NOTICES.md](./THIRD_PARTY_NOTICES.md)를 확인하세요.

