package wechat

import (
	"fmt"
	"strings"
)

// ExtractText returns the user-visible text from a message (text + voice transcription + quotes).
// Non-text media returns a placeholder notice (phase 1: no CDN download).
func ExtractText(msg Message) string {
	if text := extractTextItems(msg.ItemList); text != "" {
		return text
	}
	return mediaPlaceholder(msg.ItemList)
}

func extractTextItems(items []MessageItem) string {
	if len(items) == 0 {
		return ""
	}
	for _, item := range items {
		if item.Type == MessageItemTypeText && item.TextItem != nil && item.TextItem.Text != "" {
			text := item.TextItem.Text
			if item.RefMsg == nil {
				return text
			}
			var parts []string
			if item.RefMsg.Title != "" {
				parts = append(parts, item.RefMsg.Title)
			}
			if item.RefMsg.MessageItem != nil && item.RefMsg.MessageItem.TextItem != nil {
				if t := item.RefMsg.MessageItem.TextItem.Text; t != "" {
					parts = append(parts, t)
				}
			}
			if len(parts) == 0 {
				return text
			}
			return fmt.Sprintf("[引用: %s]\n%s", strings.Join(parts, " | "), text)
		}
		if item.Type == MessageItemTypeVoice && item.VoiceItem != nil && item.VoiceItem.Text != "" {
			return item.VoiceItem.Text
		}
	}
	return ""
}

func mediaPlaceholder(items []MessageItem) string {
	for _, item := range items {
		switch item.Type {
		case MessageItemTypeImage:
			return "[image]"
		case MessageItemTypeVoice:
			return "[voice]"
		case MessageItemTypeFile:
			return "[file]"
		case MessageItemTypeVideo:
			return "[video]"
		}
	}
	return ""
}
