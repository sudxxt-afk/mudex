package library

import (
	"context"
	"encoding/binary"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FastDuration parses duration from file headers without decoding or
// spawning processes (microseconds, safe to call while browsing).
// Returns 0 when the format needs a full scan — use ProbeDuration then.
func FastDuration(path string) time.Duration {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".wav":
		return wavDuration(path)
	case ".flac":
		return flacDuration(path)
	case ".mp3":
		return mp3Duration(path)
	}
	return 0
}

// ProbeDuration returns audio duration via ffprobe (header read).
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

// wavDuration reads RIFF fmt + data chunks (PCM only).
func wavDuration(path string) time.Duration {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	var riff [12]byte
	if _, err := io.ReadFull(f, riff[:]); err != nil {
		return 0
	}
	if string(riff[:4]) != "RIFF" || string(riff[8:12]) != "WAVE" {
		return 0
	}
	var byteRate uint32
	for {
		var ch [8]byte
		if _, err := io.ReadFull(f, ch[:]); err != nil {
			return 0
		}
		size := binary.LittleEndian.Uint32(ch[4:8])
		switch string(ch[:4]) {
		case "fmt ":
			body := make([]byte, size)
			if _, err := io.ReadFull(f, body); err != nil || len(body) < 16 {
				return 0
			}
			if binary.LittleEndian.Uint16(body[:2]) != 1 {
				return 0 // non-PCM
			}
			byteRate = binary.LittleEndian.Uint32(body[8:12])
		case "data":
			if byteRate == 0 {
				return 0
			}
			return time.Duration(float64(size) / float64(byteRate) * float64(time.Second))
		default:
			if size > 1<<28 {
				return 0
			}
			if _, err := f.Seek(int64(size), io.SeekCurrent); err != nil {
				return 0
			}
		}
	}
}

// flacDuration reads total samples from the STREAMINFO metadata block.
func flacDuration(path string) time.Duration {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	var magic [4]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil || string(magic[:]) != "fLaC" {
		return 0
	}
	for {
		var h [4]byte
		if _, err := io.ReadFull(f, h[:]); err != nil {
			return 0
		}
		last := h[0]&0x80 != 0
		typ := h[0] & 0x7F
		size := uint32(h[1])<<16 | uint32(h[2])<<8 | uint32(h[3])
		if typ == 0 { // STREAMINFO
			if size < 34 {
				return 0
			}
			body := make([]byte, 34)
			if _, err := io.ReadFull(f, body); err != nil {
				return 0
			}
			rate := uint32(body[10])<<12 | uint32(body[11])<<4 | uint32(body[12])>>4
			samples := uint64(body[13]&0x0F)<<32 | uint64(body[14])<<24 | uint64(body[15])<<16 | uint64(body[16])<<8 | uint64(body[17])
			if rate == 0 || samples == 0 {
				return 0
			}
			return time.Duration(float64(samples) / float64(rate) * float64(time.Second))
		}
		if _, err := f.Seek(int64(size), io.SeekCurrent); err != nil {
			return 0
		}
		if last {
			return 0
		}
	}
}

// mp3 bitrate table [version][layer][index]: 0 = free/bad.
var mp3Bitrates = [...][3][16]int{
	{ // MPEG1
		{0, 32, 64, 96, 128, 160, 192, 224, 256, 288, 320, 352, 384, 416, 448, 0}, // Layer I
		{0, 32, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384, 0},    // Layer II
		{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0},     // Layer III
	},
	{ // MPEG2/2.5
		{0, 32, 48, 56, 64, 80, 96, 112, 128, 144, 160, 176, 192, 224, 256, 0},
		{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
		{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
	},
}

var mp3SampleRates = [...][4]int{
	{44100, 48000, 32000, 0}, // MPEG1
	{22050, 24000, 16000, 0}, // MPEG2
	{11025, 12000, 8000, 0},  // MPEG2.5
}

// mp3Duration estimates duration: Xing/Info header when present,
// else CBR estimate from the first frame + file size.
func mp3Duration(path string) time.Duration {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	// skip ID3v2 tag
	off := int64(0)
	var pre [10]byte
	if _, err := io.ReadFull(f, pre[:]); err != nil {
		return 0
	}
	if string(pre[:3]) == "ID3" {
		size := int64(pre[6]&0x7F)<<21 | int64(pre[7]&0x7F)<<14 | int64(pre[8]&0x7F)<<7 | int64(pre[9]&0x7F)
		off = 10 + size
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			return 0
		}
	} else {
		off = 0
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return 0
		}
	}
	// first frame header (sync hunt, bounded)
	head := make([]byte, 4)
	frameOff := int64(-1)
	var ver, layer byte
	var bitrate, rate int
	for i := 0; i < 8192; i++ {
		if _, err := io.ReadFull(f, head[:2]); err != nil {
			return 0
		}
		if head[0] != 0xFF || head[1]&0xE0 != 0xE0 {
			if _, err := f.Seek(off+int64(i)+1, io.SeekStart); err != nil {
				return 0
			}
			continue
		}
		if _, err := io.ReadFull(f, head[2:]); err != nil {
			return 0
		}
		v := (head[1] >> 3) & 0x03
		l := (head[1] >> 1) & 0x03
		bi := (head[2] >> 4) & 0x0F
		ri := (head[2] >> 2) & 0x03
		if v == 1 || l == 0 || bi == 0 || bi == 15 || ri == 3 {
			if _, err := f.Seek(off+int64(i)+1, io.SeekStart); err != nil {
				return 0
			}
			continue
		}
		var li int
		switch l {
		case 3: // Layer I
			li = 0
		case 2: // Layer II
			li = 1
		default: // Layer III
			li = 2
		}
		bitrate = mp3Bitrates[mapVer(v)][li][bi]
		rate = mp3SampleRates[mapVerRate(v)][ri]
		if bitrate == 0 || rate == 0 {
			return 0
		}
		frameOff = off + int64(i)
		ver = v
		layer = l
		break
	}
	if frameOff < 0 {
		return 0
	}
	// Xing/Info header check (side info size depends on version/channels)
	ch := (head[3] >> 6) & 0x03
	mpeg1 := ver == 3
	sideLen := 32
	if !mpeg1 {
		sideLen = 17
	}
	if ch == 3 { // mono
		if mpeg1 {
			sideLen = 17
		} else {
			sideLen = 9
		}
	}
	xingAt := frameOff + 4 + int64(sideLen)
	if _, err := f.Seek(xingAt, io.SeekStart); err == nil {
		var tag [12]byte
		if _, err := io.ReadFull(f, tag[:]); err == nil {
			if string(tag[:4]) == "Xing" || string(tag[:4]) == "Info" {
				flags := binary.BigEndian.Uint32(tag[4:8])
				if flags&1 != 0 {
					frames := binary.BigEndian.Uint32(tag[8:12])
					spb := 1152
					if layer == 3 { // Layer I
						spb = 384
					} else if !mpeg1 && layer == 1 { // MPEG2 Layer III
						spb = 576
					}
					return time.Duration(float64(frames) * float64(spb) / float64(rate) * float64(time.Second))
				}
			}
		}
	}
	// CBR fallback: audio bytes / bitrate
	audio := st.Size() - frameOff
	sec := float64(audio*8) / float64(bitrate*1000)
	if sec <= 0 || sec > 24*3600 {
		return 0
	}
	return time.Duration(sec * float64(time.Second))
}

func mapVer(v byte) int {
	if v == 3 {
		return 0
	}
	return 1
}

func mapVerRate(v byte) int {
	switch v {
	case 3:
		return 0
	case 2:
		return 1
	default:
		return 2
	}
}
