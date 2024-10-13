package clipboard

import (
	"github.com/atotto/clipboard"
)

type ClipboardUtil struct{}

func NewClipboardUtil() *ClipboardUtil {
	return &ClipboardUtil{}
}

func (cu *ClipboardUtil) Copy(text string) error {
	return clipboard.WriteAll(text)
}
