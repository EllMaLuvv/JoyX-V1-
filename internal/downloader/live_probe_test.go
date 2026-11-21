package downloader

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLiveProbe downloads real posts through the exact path /download uses. It
// is off unless LIVE_PROBE=1, because it needs the network and a logged-in
// browser profile (TikTok and Instagram refuse anonymous datacenter requests).
//
//	LOCALAPPDATA=... LIVE_PROBE=1 go test ./internal/downloader -run TestLiveProbe -v
func TestLiveProbe(t *testing.T) {
	if os.Getenv("LIVE_PROBE") == "" {
		t.Skip("set LIVE_PROBE=1 to run a real download")
	}
	urls := []string{
		os.Getenv("PROBE_TIKTOK"),
		os.Getenv("PROBE_X"),
	}
	for _, extra := range strings.Split(os.Getenv("PROBE_URLS"), "|") {
		if extra != "" {
			urls = append(urls, extra)
		}
	}
	for _, rawURL := range urls {
		if rawURL == "" {
			continue
		}
		t.Run(filepath.Base(rawURL), func(t *testing.T) {
			media, err := Fetch(context.Background(), rawURL, Options{
				YtDlpPath:          os.Getenv("PROBE_YTDLP"),
				FFmpegPath:         os.Getenv("PROBE_FFMPEG"),
				CookiesFromBrowser: os.Getenv("PROBE_COOKIES_FROM"),
				MaxBytes:           200 << 20,
				Timeout:            5 * time.Minute,
			})
			if err != nil {
				t.Fatalf("fetch: %v", err)
			}
			defer os.RemoveAll(filepath.Dir(media.Path))
			info, err := os.Stat(media.Path)
			if err != nil {
				t.Fatalf("stat: %v", err)
			}
			t.Logf("title=%q size=%d resolution=%dx%d fps=%.0f mimetype=%s",
				media.Title, info.Size(), media.Width, media.Height, media.FPS, media.Mimetype)
			if info.Size() == 0 {
				t.Fatal("downloaded an empty file")
			}
			if media.Width == 0 || media.Height == 0 {
				t.Fatalf("no video dimensions reported: %+v", media)
			}
			if media.Mimetype != "video/mp4" {
				t.Fatalf("expected an mp4, got %s", media.Mimetype)
			}
		})
	}
}
