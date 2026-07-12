package util

import (
	"fmt"
	"path/filepath"
	"strings"
)

func Xor32(value int64, key uint32) int64 {
	return int64(uint32(value) ^ key)
}

func CleanEntryName(name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return clean, nil
}
