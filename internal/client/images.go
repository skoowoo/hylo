package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/hardhacker/hylo/internal/storage"
)

// UploadedImage is a stored vault image: its asset path and the name that
// ![[name]] embeds it by.
type UploadedImage struct {
	Src  string `json:"src"`
	Name string `json:"name"`
}

// UploadImage sends a local image file to the vault's image store.
func (c *Client) UploadImage(path string) (UploadedImage, error) {
	st, err := os.Stat(path)
	if err != nil {
		return UploadedImage{}, fmt.Errorf("image %q: %w", path, err)
	}
	if st.IsDir() {
		return UploadedImage{}, fmt.Errorf("image %q is a directory", path)
	}
	if st.Size() > storage.MaxImageBytes {
		return UploadedImage{}, fmt.Errorf("image %q is larger than %d MB", path, storage.MaxImageBytes>>20)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return UploadedImage{}, fmt.Errorf("image %q: %w", path, err)
	}
	return c.uploadImage(func(w *multipart.Writer) error {
		part, err := w.CreateFormFile("file", filepath.Base(path))
		if err != nil {
			return err
		}
		_, err = part.Write(data)
		return err
	})
}

// UploadChatImage stores an image the user attached in chat, named by its
// file name in the upload folder; the server reads nothing outside that folder.
func (c *Client) UploadChatImage(name string) (UploadedImage, error) {
	return c.uploadImage(func(w *multipart.Writer) error {
		return w.WriteField("chat_upload", name)
	})
}

func (c *Client) uploadImage(fill func(*multipart.Writer) error) (UploadedImage, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := fill(w); err != nil {
		return UploadedImage{}, err
	}
	if err := w.Close(); err != nil {
		return UploadedImage{}, err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/vault/upload-image", &buf)
	if err != nil {
		return UploadedImage{}, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := c.do(req)
	if err != nil {
		return UploadedImage{}, wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return UploadedImage{}, fmt.Errorf("upload image: %s", statusMsg(resp.StatusCode, raw))
	}
	var out UploadedImage
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return UploadedImage{}, fmt.Errorf("upload image: decode response: %w", err)
	}
	return out, nil
}
