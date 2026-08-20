package downloader

import "testing"

func TestValidateYouTubeURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"watch", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", false},
		{"short link", "https://youtu.be/dQw4w9WgXcQ", false},
		{"shorts", "https://youtube.com/shorts/dQw4w9WgXcQ", false},
		{"music", "https://music.youtube.com/watch?v=dQw4w9WgXcQ", false},
		{"empty", "", true},
		{"no scheme", "youtube.com/watch?v=abc", true},
		{"wrong host", "https://example.com/watch?v=abc", true},
		{"spoof host", "https://youtube.com.example.org/watch?v=abc", true},
		{"home page", "https://youtube.com/", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateYouTubeURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateYouTubeURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestFriendlyError(t *testing.T) {
	got := friendlyError("This is a private video")
	if got != "비공개 영상은 다운로드할 수 없습니다" {
		t.Fatalf("unexpected friendly error: %q", got)
	}
}
