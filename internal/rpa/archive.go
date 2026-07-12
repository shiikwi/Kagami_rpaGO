package rpa

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"

	"kagami_rpago/internal/pickle"
	"kagami_rpago/internal/util"
)

func Open(path string) (*Xp3rpaArchive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	header := make([]byte, 40)
	io.ReadFull(f, header)

	if string(header[:8]) != "RPA-3.0 " {
		return nil, fmt.Errorf("invalid header %q", string(header[:8]))
	}

	indexOffset, err := strconv.ParseInt(string(header[8:24]), 16, 64)
	if err != nil {
		return nil, fmt.Errorf("parse index offset: %w", err)
	}
	keyValue, err := strconv.ParseUint(string(header[25:33]), 16, 32)
	if err != nil {
		return nil, fmt.Errorf("parse key: %w", err)
	}

	indexData, err := readIndex(f, indexOffset)
	if err != nil {
		return nil, err
	}

	root, err := pickle.Parse(indexData)
	if err != nil {
		return nil, fmt.Errorf("parse pickle index: %w", err)
	}

	index := root.(pickle.Dict)

	entries, err := decodeEntries(index, uint32(keyValue), info.Size(), indexOffset)
	if err != nil {
		return nil, err
	}

	return &Xp3rpaArchive{
		Path:        path,
		IndexOffset: indexOffset,
		Key:         uint32(keyValue),
		Entries:     entries,
	}, nil
}

func readIndex(f *os.File, indexOffset int64) ([]byte, error) {
	f.Seek(indexOffset, io.SeekStart)
	compressed, _ := io.ReadAll(f)

	zr, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("init zlib fail: %w", err)
	}
	defer zr.Close()

	indexData, _ := io.ReadAll(zr)
	return indexData, nil
}

func decodeEntries(index pickle.Dict, key uint32, fileSize, indexOffset int64) ([]FileEntry, error) {
	entries := make([]FileEntry, 0, len(index))
	for rawName, value := range index {
		name, err := util.CleanEntryName(rawName)
		if err != nil {
			return nil, err
		}

		list := value.(*pickle.List)

		segments := make([]Segment, 0, len(*list))
		for i, rawSegment := range *list {
			tuple := rawSegment.(pickle.Tuple)

			segment, err := decodeSegment(tuple, key, fileSize, indexOffset)
			if err != nil {
				return nil, fmt.Errorf("entry %s segment %d: %w", rawName, i, err)
			}
			segments = append(segments, segment)
		}

		entries = append(entries, FileEntry{Name: name, Segments: segments})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

func decodeSegment(tuple pickle.Tuple, key uint32, fileSize, indexOffset int64) (Segment, error) {
	first := asInt64(tuple[0])
	second := asInt64(tuple[1])
	prefix := asBytes(tupleValue(tuple, 2))

	official := Segment{
		Offset: util.Xor32(second, key),
		Length: util.Xor32(first, key),
		Prefix: prefix,
	}
	swapped := Segment{
		Offset: util.Xor32(first, key),
		Length: util.Xor32(second, key),
		Prefix: prefix,
	}

	switch {
	case segmentLooksValid(official, fileSize, indexOffset):
		return official, nil
	case segmentLooksValid(swapped, fileSize, indexOffset):
		return swapped, nil
	default:
		return Segment{}, fmt.Errorf(
			"no sane offset/length candidate: official=(offset=%d,length=%d) swapped=(offset=%d,length=%d)",
			official.Offset,
			official.Length,
			swapped.Offset,
			swapped.Length,
		)
	}
}

func segmentLooksValid(segment Segment, fileSize, indexOffset int64) bool {
	if segment.Offset < 40 || segment.Length < 0 {
		return false
	}
	end := segment.Offset + segment.Length
	if end < segment.Offset || end > fileSize {
		return false
	}
	return segment.Offset < indexOffset && end <= indexOffset
}

func asInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	default:
		return 0
	}
}

func asBytes(v any) []byte {
	switch x := v.(type) {
	case nil:
		return nil
	case []byte:
		return append([]byte(nil), x...)
	case string:
		return []byte(x)
	default:
		return nil
	}
}

func tupleValue(tuple pickle.Tuple, i int) any {
	if i >= len(tuple) {
		return nil
	}
	return tuple[i]
}
