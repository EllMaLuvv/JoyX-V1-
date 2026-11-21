package downloader

import "testing"

func TestAllowedHost(t *testing.T) {
	cases := map[string]bool{
		"https://www.tiktok.com/@zachking/video/6768504823336815877": true,
		"https://vm.tiktok.com/ZM8abcDEf/":                           true,
		"https://vt.tiktok.com/ZM8abcDEf/":                           true,
		"https://www.instagram.com/reel/C8Y1nKtR1aB/":                true,
		"https://instagr.am/p/C8Y1nKtR1aB/":                          true,
		"https://www.facebook.com/watch/?v=465569335043867":          true,
		"https://fb.watch/abc123/":                                   true,
		"https://www.youtube.com/watch?v=jNQXAC9IVRw":                false,
		"https://example.com/video":                                  false,
		"not a url":                                                  false,
		"":                                                           false,
	}
	for rawURL, want := range cases {
		if got := AllowedHost(rawURL); got != want {
			t.Fatalf("AllowedHost(%q) = %v; want %v", rawURL, got, want)
		}
	}
}

func infoWith(formats ...ytdlpFormat) ytdlpInfo {
	return ytdlpInfo{Title: "Video", Formats: formats}
}

func TestChooseFormatPicks1080p60WhenItExists(t *testing.T) {
	video, audio := chooseFormat(infoWith(
		ytdlpFormat{FormatID: "1080-30", VCodec: "avc1", Width: 1920, Height: 1080, FPS: 30, Filesize: 1_000_000},
		ytdlpFormat{FormatID: "1080-60", VCodec: "avc1", Width: 1920, Height: 1080, FPS: 60, Filesize: 1_500_000},
		ytdlpFormat{FormatID: "720-60", VCodec: "avc1", Width: 1280, Height: 720, FPS: 60, Filesize: 800_000},
		ytdlpFormat{FormatID: "opus", ACodec: "opus", ABR: 128},
	))
	if video.FormatID != "1080-60" {
		t.Fatalf("video = %+v", video)
	}
	if audio != "opus" {
		t.Fatalf("audio = %q", audio)
	}
}

func TestChooseFormatStaysWithinTheCeiling(t *testing.T) {
	video, _ := chooseFormat(infoWith(
		ytdlpFormat{FormatID: "4k-60", VCodec: "av01", Width: 3840, Height: 2160, FPS: 60, Filesize: 9_000_000},
		ytdlpFormat{FormatID: "1080-30", VCodec: "avc1", Width: 1920, Height: 1080, FPS: 30, Filesize: 1_000_000},
		ytdlpFormat{FormatID: "720-30", VCodec: "avc1", Width: 1280, Height: 720, FPS: 30, Filesize: 500_000},
	))
	if video.FormatID != "1080-30" {
		t.Fatalf("video = %+v", video)
	}
}

func TestChooseFormatPrefersTheBestFrameRateUnderTheCap(t *testing.T) {
	video, _ := chooseFormat(infoWith(
		ytdlpFormat{FormatID: "1080-120", VCodec: "avc1", Width: 1920, Height: 1080, FPS: 120, Filesize: 3_000_000},
		ytdlpFormat{FormatID: "1080-60", VCodec: "avc1", Width: 1920, Height: 1080, FPS: 60, Filesize: 1_500_000},
	))
	if video.FormatID != "1080-60" {
		t.Fatalf("video = %+v", video)
	}
}

func TestChooseFormatKeepsAudioAlreadyInTheVideo(t *testing.T) {
	video, audio := chooseFormat(infoWith(
		ytdlpFormat{FormatID: "progressive", VCodec: "avc1", ACodec: "mp4a", Width: 1280, Height: 720, FPS: 30, Filesize: 1_000_000},
		ytdlpFormat{FormatID: "opus", ACodec: "opus", ABR: 128},
	))
	if video.FormatID != "progressive" || audio != "" {
		t.Fatalf("video = %+v audio = %q", video, audio)
	}
}

func TestChooseFormatFallsBackToTheOnlyVideo(t *testing.T) {
	video, audio := chooseFormat(infoWith(
		ytdlpFormat{FormatID: "4k-only", VCodec: "av01", Width: 3840, Height: 2160, FPS: 60, Filesize: 9_000_000},
	))
	if video.FormatID != "4k-only" || audio != "" {
		t.Fatalf("video = %+v audio = %q", video, audio)
	}
}
