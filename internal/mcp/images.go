package mcp

import (
	"context"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/notes"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type imageUploadReq struct {
	Path string `json:"path" jsonschema:"file path of an image the user attached in this chat, as you were given it. For other image files run hylo image upload"`
}

type imageUploadOut struct {
	Name  string `json:"name"`
	Src   string `json:"src"`
	Embed string `json:"embed" jsonschema:"markdown that shows the image in a note"`
}

func registerImageTools(s *sdk.Server, c *client.Client) {
	addTool(s, "image_upload", "Save an image the user attached in chat into the vault and get the ![[name]] embed to put in a note. For shorts, pass images to short_create instead.", additive,
		func(_ context.Context, in imageUploadReq) (imageUploadOut, error) {
			imgs, err := notes.UploadChatImages(c, []string{in.Path})
			if err != nil {
				return imageUploadOut{}, err
			}
			return imageUploadOut{Name: imgs[0].Name, Src: imgs[0].Src, Embed: notes.ImageEmbed(imgs[0].Name)}, nil
		})
}
