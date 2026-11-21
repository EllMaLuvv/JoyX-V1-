package hypermeow

import (
	"bytes"
	"context"
	"errors"
	"image"

	_ "image/gif" // registers the GIF decoder for image config
	_ "image/jpeg"
	_ "image/png"

	whatsmeow "github.com/polymorfa/hypermeow"
	"github.com/polymorfa/hypermeow/proto/waE2E"
	"github.com/polymorfa/hypermeow/types"
	"google.golang.org/protobuf/proto"

	"github.com/gen2brain/webp"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/identity"
)

// maxSentMediaBytes bounds what /img and /rvo may upload.
const maxSentMediaBytes = 64 << 20

// SendImage uploads data and sends it as an image, quoting quoted when set.
func (adapter *Adapter) SendImage(ctx context.Context, key agent.Key, data []byte, mimetype string, quoted identity.MessageID) error {
	if len(data) == 0 {
		return agent.NewError(agent.ErrorInvalidArgument, "send WhatsApp image", errors.New("image is empty"))
	}
	if len(data) > maxSentMediaBytes {
		return agent.NewError(agent.ErrorInvalidArgument, "send WhatsApp image", errors.New("image is too large"))
	}
	if mimetype == "" {
		mimetype = "image/png"
	}
	width, height := imageDimensions(data)
	_, err := adapter.send(ctx, key, "send WhatsApp image", func(sendCtx context.Context, address string, target types.JID) (*waE2E.Message, error) {
		contextInfo, err := adapter.quoteContext(sendCtx, key, address, target, quoted)
		if err != nil {
			return nil, err
		}
		uploaded, err := adapter.client.Upload(sendCtx, data, whatsmeow.MediaImage)
		if err != nil {
			return nil, nativeEffectError(sendCtx, "upload WhatsApp image", err)
		}
		return &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uploaded.FileLength),
			Mimetype:      proto.String(mimetype),
			Width:         proto.Uint32(uint32(width)),
			Height:        proto.Uint32(uint32(height)),
			ContextInfo:   contextInfo,
		}}, nil
	})
	return err
}

// SendVideo uploads data and sends it as a video, quoting quoted when set.
func (adapter *Adapter) SendVideo(ctx context.Context, key agent.Key, data []byte, mimetype string, quoted identity.MessageID) error {
	if len(data) == 0 {
		return agent.NewError(agent.ErrorInvalidArgument, "send WhatsApp video", errors.New("video is empty"))
	}
	if len(data) > maxSentMediaBytes {
		return agent.NewError(agent.ErrorInvalidArgument, "send WhatsApp video", errors.New("video is too large"))
	}
	if mimetype == "" {
		mimetype = "video/mp4"
	}
	_, err := adapter.send(ctx, key, "send WhatsApp video", func(sendCtx context.Context, address string, target types.JID) (*waE2E.Message, error) {
		contextInfo, err := adapter.quoteContext(sendCtx, key, address, target, quoted)
		if err != nil {
			return nil, err
		}
		uploaded, err := adapter.client.Upload(sendCtx, data, whatsmeow.MediaVideo)
		if err != nil {
			return nil, nativeEffectError(sendCtx, "upload WhatsApp video", err)
		}
		return &waE2E.Message{VideoMessage: &waE2E.VideoMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uploaded.FileLength),
			Mimetype:      proto.String(mimetype),
			ContextInfo:   contextInfo,
		}}, nil
	})
	return err
}

// imageDimensions reads the width and height of a PNG, JPEG, GIF or WebP
// without decoding the whole image. It returns zero when the format is
// unknown, which WhatsApp accepts.
func imageDimensions(data []byte) (int, int) {
	if isWebPBytes(data) {
		if config, err := webp.DecodeConfig(bytes.NewReader(data)); err == nil {
			return config.Width, config.Height
		}
		return 0, 0
	}
	if config, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		return config.Width, config.Height
	}
	return 0, 0
}

func isWebPBytes(data []byte) bool {
	return len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}
