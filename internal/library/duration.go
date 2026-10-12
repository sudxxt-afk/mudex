package library

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ProbeDuration returns audio duration via ffprobe (header read, fast).
// Returns 0 when ffprobe is missing or probing fails — callers show "--:--".
func ProbeDuration(path string) time.Duration {
	fp, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, fp, "-v", "error",
		"-show_entries", "format=duration", "-of", "csv=p=0", path).Output()
	if err != nil {
		return 0
	}
	sec, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || sec <= 0 {
		return 0
	}
	return time.Duration(sec * float64(time.Second))
}
