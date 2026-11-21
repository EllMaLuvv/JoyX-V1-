package inbound

import (
	"errors"
	"testing"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/command"
)

func TestFirstSupportedLink(t *testing.T) {
	cases := []struct {
		text string
		want string
		ok   bool
	}{
		{"https://www.tiktok.com/@zachking/video/6768504823336815877", "https://www.tiktok.com/@zachking/video/6768504823336815877", true},
		{"nih https://vm.tiktok.com/ZM8abcDEf/", "https://vm.tiktok.com/ZM8abcDEf/", true},
		{"https://www.instagram.com/reel/C8Y1nKtR1aB/?igsh=MWQ1", "https://www.instagram.com/reel/C8Y1nKtR1aB/?igsh=MWQ1", true},
		{"https://fb.watch/abc123/?ref=share", "https://fb.watch/abc123/?ref=share", true},
		{"(https://www.facebook.com/watch/?v=465569335043867)", "https://www.facebook.com/watch/?v=465569335043867", true},
		{"yang ini https://www.youtube.com/watch?v=jNQXAC9IVRw bagus", "", false},
		{"https://example.com/video", "", false},
		{"nih @budi coba", "", false},
		{"", "", false},
	}
	for _, testCase := range cases {
		got, ok := firstSupportedLink(testCase.text)
		if ok != testCase.ok || got != testCase.want {
			t.Fatalf("firstSupportedLink(%q) = %q, %v; want %q, %v", testCase.text, got, ok, testCase.want, testCase.ok)
		}
	}
}

func TestDownloadCaptionAndFailure(t *testing.T) {
	media := command.RemoteMedia{Title: "Judul Video", Width: 1920, Height: 1080, FPS: 60}
	if caption := downloadCaption(media); caption != "Judul Video\n1920x1080 60fps" {
		t.Fatalf("caption = %q", caption)
	}
	if label := downloadResolutionLabel(1280, 720, 0); label != "1280x720" {
		t.Fatalf("label = %q", label)
	}
	message := downloadFailure(errors.New("download video: login required"))
	if message != "login required." {
		t.Fatalf("failure = %q", message)
	}
}
