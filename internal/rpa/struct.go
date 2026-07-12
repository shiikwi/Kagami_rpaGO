package rpa

type Xp3rpaArchive struct {
	Path        string
	IndexOffset int64
	Key         uint32
	Entries     []FileEntry
}

type FileEntry struct {
	Name     string
	Segments []Segment
}

type Segment struct {
	Offset int64
	Length int64
	Prefix []byte
}

func (entry FileEntry) Size() int64 {
	var size int64
	for _, segment := range entry.Segments {
		size += int64(len(segment.Prefix)) + segment.Length
	}
	return size
}
