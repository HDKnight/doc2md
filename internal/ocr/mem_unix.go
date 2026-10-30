//go:build unix

package ocr

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func systemMemory() (total, avail uint64, ok bool) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var memTotal, memAvail uint64
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			memTotal = parseMeminfoKB(line)
		case strings.HasPrefix(line, "MemAvailable:"):
			memAvail = parseMeminfoKB(line)
		}
	}
	if memTotal == 0 {
		return 0, 0, false
	}
	if memAvail == 0 {
		memAvail = memTotal / 2
	}
	return memTotal * 1024, memAvail * 1024, true
}

func parseMeminfoKB(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	n, _ := strconv.ParseUint(fields[1], 10, 64)
	return n
}
