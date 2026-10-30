//go:build !windows && !unix

package ocr

func systemMemory() (total, avail uint64, ok bool) {
	return 0, 0, false
}
