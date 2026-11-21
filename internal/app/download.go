package app

import (
	"context"

	whatsapp "github.com/Chomosuke9/WazzapAgent-Go/internal/adapters/whatsapp/hypermeow"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/command"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/downloader"
)

// remoteMediaFetcher adapts the downloader package to the command port, so
// /download never imports yt-dlp or knows where the tools live.
type remoteMediaFetcher struct {
	options downloader.Options
}

func (fetcher remoteMediaFetcher) Fetch(ctx context.Context, rawURL string) (command.RemoteMedia, error) {
	media, err := downloader.Fetch(ctx, rawURL, fetcher.options)
	if err != nil {
		return command.RemoteMedia{}, err
	}
	return command.RemoteMedia{
		Path: media.Path, Title: media.Title, Width: media.Width, Height: media.Height,
		FPS: media.FPS, Mimetype: media.Mimetype,
	}, nil
}

// fileSender adapts the WhatsApp adapter's document sending to the command port.
// An empty Kind is sent as a document, which is what keeps a download's quality.
type fileSender struct {
	adapter *whatsapp.Adapter
}

func (sender fileSender) SendFile(ctx context.Context, key agent.Key, file command.OutboundFile) (string, error) {
	return sender.adapter.SendFile(ctx, key, whatsapp.OutboundFile{
		Path: file.Path, Name: file.Name, Mime: file.Mime, Kind: file.Kind,
	})
}
