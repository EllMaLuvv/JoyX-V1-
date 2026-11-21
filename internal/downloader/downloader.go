// Package downloader fetches a public video through yt-dlp and hands back a
// local file. It knows nothing about WhatsApp: commands send what it returns.
//
// yt-dlp, not this package, decides what a site serves. TikTok and Instagram
// usually refuse a datacenter address that is not logged in, so Options carries
// an optional browser or cookie file to authenticate with.
package downloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
)

const (
	// MaxHeight is the ceiling /download asks for: 1080p.
	MaxHeight = 1080
	// MaxFPS caps the frame rate, so a 1080p60 source is picked over a 120fps
	// one but a 1080p30 source is never rejected for being lower.
	MaxFPS = 60
	// DefaultTimeout bounds one download.
	DefaultTimeout = 10 * time.Minute
)

// allowedHosts are the platforms /download accepts.
var allowedHosts = map[string]bool{
	"tiktok.com": true, "www.tiktok.com": true, "m.tiktok.com": true,
	"vm.tiktok.com": true, "vt.tiktok.com": true,
	"instagram.com": true, "www.instagram.com": true, "m.instagram.com": true,
	"instagr.am":   true,
	"facebook.com": true, "www.facebook.com": true, "m.facebook.com": true,
	"fb.watch": true, "fb.com": true,
	"x.com": true, "www.x.com": true, "mobile.x.com": true,
	"twitter.com": true, "www.twitter.com": true, "mobile.twitter.com": true,
}

// Options configures one fetch.
type Options struct {
	// YtDlpPath and FFmpegPath are the external tools. FFmpegPath may be empty,
	// which lets yt-dlp look on PATH.
	YtDlpPath  string
	FFmpegPath string
	// MaxBytes refuses a finished file larger than this.
	MaxBytes int64
	// Timeout bounds the whole fetch. Zero means DefaultTimeout.
	Timeout time.Duration
	// CookiesFromBrowser authenticates yt-dlp from a logged-in browser, which
	// TikTok and Instagram generally require. Empty means no cookies.
	CookiesFromBrowser string
	// CookiesFile is a Netscape cookie file, used when CookiesFromBrowser is
	// empty and a browser session is not wanted.
	CookiesFile string
}

// Media is a downloaded video ready to send.
type Media struct {
	Path     string
	Title    string
	Width    int
	Height   int
	FPS      float64
	Mimetype string
}

// AllowedHost reports whether rawURL is a TikTok, Instagram or Facebook link.
func AllowedHost(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	return allowedHosts[strings.ToLower(parsed.Host)]
}

// Fetch downloads rawURL and returns the file. The caller removes Path.
func Fetch(ctx context.Context, rawURL string, options Options) (Media, error) {
	rawURL = strings.TrimSpace(rawURL)
	if !AllowedHost(rawURL) {
		return Media{}, agent.NewError(agent.ErrorInvalidArgument, "download video",
			errors.New("only TikTok, Instagram and Facebook links are supported"))
	}
	if options.YtDlpPath == "" {
		options.YtDlpPath = "yt-dlp"
	}
	if options.Timeout <= 0 {
		options.Timeout = DefaultTimeout
	}
	if options.MaxBytes <= 0 {
		options.MaxBytes = 200 << 20
	}
	fetchCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	directory, err := os.MkdirTemp("", "wazzapagent-download-*")
	if err != nil {
		return Media{}, agent.NewError(agent.ErrorStorageFailure, "create download directory", err)
	}
	defer func() {
		// A caller that got a Media owns the file; on failure the directory is
		// worthless, so it always goes.
		if err != nil {
			_ = os.RemoveAll(directory)
		}
	}()

	info, err := probe(fetchCtx, rawURL, options)
	if err != nil {
		return Media{}, err
	}
	video, audio := chooseFormat(info)
	path, err := download(fetchCtx, rawURL, options, directory, video.FormatID, audio)
	if err != nil {
		return Media{}, err
	}
	size, err := fileSize(path)
	if err != nil {
		_ = os.RemoveAll(directory)
		return Media{}, err
	}
	if size > options.MaxBytes {
		_ = os.RemoveAll(directory)
		return Media{}, agent.NewError(agent.ErrorInvalidArgument, "download video",
			fmt.Errorf("the video is %.1f MB, over the %.1f MB limit", float64(size)/(1<<20), float64(options.MaxBytes)/(1<<20)))
	}
	mimetype := mediaMimetype(path)
	return Media{
		Path: path, Title: info.Title, Width: video.Width, Height: video.Height, FPS: video.FPS,
		Mimetype: mimetype,
	}, nil
}

// mediaMimetype reads the type from the finished file's extension, which is the
// only honest answer once yt-dlp may have merged, converted or simply saved a
// photo. The caller chooses image or video from it.
func mediaMimetype(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "video/mp4"
	}
}

// ytdlpFormat is one stream yt-dlp reported.
type ytdlpFormat struct {
	FormatID string  `json:"format_id"`
	VCodec   string  `json:"vcodec"`
	ACodec   string  `json:"acodec"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	FPS      float64 `json:"fps"`
	ABR      float64 `json:"abr"`
	TBR      float64 `json:"tbr"`
	Ext      string  `json:"ext"`
	Filesize int64   `json:"filesize"`
}

// ytdlpInfo is the metadata yt-dlp dumps before downloading.
type ytdlpInfo struct {
	Title   string        `json:"title"`
	Formats []ytdlpFormat `json:"formats"`
}

func probe(ctx context.Context, rawURL string, options Options) (ytdlpInfo, error) {
	arguments := []string{
		"--dump-single-json", "--no-playlist", "--no-warnings", "--simulate", "--no-cache-dir",
	}
	arguments = append(arguments, cookieArguments(options)...)
	arguments = append(arguments, rawURL)
	output, err := run(ctx, options, arguments...)
	if err != nil {
		return ytdlpInfo{}, err
	}
	var info ytdlpInfo
	if err := json.Unmarshal([]byte(output), &info); err != nil {
		return ytdlpInfo{}, agent.NewError(agent.ErrorProviderFailure, "read yt-dlp metadata", err)
	}
	if len(info.Formats) == 0 {
		return ytdlpInfo{}, agent.NewError(agent.ErrorNotFound, "read yt-dlp metadata",
			errors.New("this link has no downloadable video"))
	}
	return info, nil
}

func download(ctx context.Context, rawURL string, options Options, directory, videoFormatID, audioFormatID string) (string, error) {
	arguments := []string{
		"--no-playlist", "--no-warnings", "--no-cache-dir",
	}
	if videoFormatID == "" {
		// Photo story / image post: one file, no merge, no height ceiling.
		arguments = append(arguments, "-f", "b")
	} else {
		selector := videoFormatID
		if audioFormatID != "" && audioFormatID != videoFormatID {
			selector = videoFormatID + "+" + audioFormatID
		}
		selector += fmt.Sprintf("/b[height<=%d]/b", MaxHeight)
		arguments = append(arguments, "-f", selector, "--merge-output-format", "mp4")
	}
	arguments = append(arguments,
		"-o", filepath.Join(directory, "media.%(ext)s"),
	)
	arguments = append(arguments, cookieArguments(options)...)
	arguments = append(arguments, rawURL)
	if _, err := run(ctx, options, arguments...); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", agent.NewError(agent.ErrorStorageFailure, "read download directory", err)
	}
	best := ""
	size := int64(-1)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.Size() <= size {
			continue
		}
		best, size = filepath.Join(directory, entry.Name()), info.Size()
	}
	if best == "" {
		return "", agent.NewError(agent.ErrorNotFound, "download video", errors.New("yt-dlp left no file behind"))
	}
	return best, nil
}

func cookieArguments(options Options) []string {
	switch {
	case strings.TrimSpace(options.CookiesFromBrowser) != "":
		return []string{"--cookies-from-browser", strings.TrimSpace(options.CookiesFromBrowser)}
	case strings.TrimSpace(options.CookiesFile) != "":
		return []string{"--cookies", strings.TrimSpace(options.CookiesFile)}
	default:
		return nil
	}
}

func run(ctx context.Context, options Options, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, options.YtDlpPath, arguments...)
	if ffmpeg := strings.TrimSpace(options.FFmpegPath); ffmpeg != "" {
		command.Args = append(command.Args, "--ffmpeg-location", filepath.Dir(ffmpeg))
	}
	var standardError strings.Builder
	command.Stderr = &standardError
	output, err := command.Output()
	if ctxErr := ctx.Err(); ctxErr != nil {
		if ctxErr == context.DeadlineExceeded {
			return "", agent.NewError(agent.ErrorTimeout, "run yt-dlp", ctxErr)
		}
		return "", agent.NewError(agent.ErrorCancelled, "run yt-dlp", ctxErr)
	}
	if err != nil {
		message := strings.TrimSpace(standardError.String())
		if message == "" {
			message = err.Error()
		}
		// yt-dlp explains a refusal (private video, login required, removed
		// post) on stderr; surface its own words rather than the exit status.
		return "", agent.NewError(agent.ErrorProviderFailure, "download video", errors.New(firstLine(message)))
	}
	return strings.TrimSpace(string(output)), nil
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	if len(line) > 400 {
		line = line[:400]
	}
	return line
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, agent.NewError(agent.ErrorStorageFailure, "inspect downloaded video", err)
	}
	return info.Size(), nil
}

// resolution grades a format by its short edge, which is what "1080p" means for
// both shapes: 1920x1080 and the portrait 1080x1920 are both 1080p.
func resolution(format ytdlpFormat) int {
	if format.Width <= 0 {
		return format.Height
	}
	if format.Height <= 0 {
		return format.Width
	}
	return min(format.Width, format.Height)
}

// chooseFormat picks the highest video at or under MaxHeight and, among those,
// the highest frame rate at or under MaxFPS, then the best separate audio. It
// returns the audio format ID "" when the chosen video already carries sound.
func chooseFormat(info ytdlpInfo) (ytdlpFormat, string) {
	videos := make([]ytdlpFormat, 0, len(info.Formats))
	for _, format := range info.Formats {
		if hasVideo(format) {
			videos = append(videos, format)
		}
	}
	if len(videos) == 0 {
		return ytdlpFormat{}, ""
	}
	within := make([]ytdlpFormat, 0, len(videos))
	for _, format := range videos {
		if size := resolution(format); size > 0 && size <= MaxHeight {
			within = append(within, format)
		}
	}
	if len(within) > 0 {
		videos = within
	}
	smooth := make([]ytdlpFormat, 0, len(videos))
	for _, format := range videos {
		if format.FPS <= 0 || format.FPS <= MaxFPS {
			smooth = append(smooth, format)
		}
	}
	if len(smooth) > 0 {
		videos = smooth
	}
	sort.SliceStable(videos, func(i, j int) bool {
		left, right := videos[i], videos[j]
		if leftResolution, rightResolution := resolution(left), resolution(right); leftResolution != rightResolution {
			return leftResolution > rightResolution
		}
		if left.FPS != right.FPS {
			return left.FPS > right.FPS
		}
		return left.Filesize > right.Filesize
	})
	video := videos[0]
	if hasAudio(video) {
		return video, ""
	}
	bestAudio := ""
	bestBitrate := float64(-1)
	for _, format := range info.Formats {
		if !hasAudio(format) || hasVideo(format) {
			continue
		}
		if bitrate := audioBitrate(format); bitrate > bestBitrate {
			bestAudio, bestBitrate = format.FormatID, bitrate
		}
	}
	return video, bestAudio
}

func hasVideo(format ytdlpFormat) bool {
	return format.VCodec != "" && format.VCodec != "none"
}

func hasAudio(format ytdlpFormat) bool {
	return format.ACodec != "" && format.ACodec != "none"
}

func audioBitrate(format ytdlpFormat) float64 {
	if format.ABR > 0 {
		return format.ABR
	}
	return format.TBR
}
