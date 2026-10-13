package library

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTestWAV(t *testing.T, path string, secs float64) {
	t.Helper()
	sr := 44100
	n := int(float64(sr) * secs)
	data := make([]byte, 44+n*4)
	copy(data, []byte("RIFF"))
	binary.LittleEndian.PutUint32(data[4:], uint32(36+n*4))
	copy(data[8:], []byte("WAVEfmt "))
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 2)
	binary.LittleEndian.PutUint32(data[24:], uint32(sr))
	binary.LittleEndian.PutUint32(data[28:], uint32(sr*4))
	binary.LittleEndian.PutUint16(data[32:], 4)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], []byte("data"))
	binary.LittleEndian.PutUint32(data[40:], uint32(n*4))
	for i := 0; i < n; i++ {
		v := int16(math.Sin(2*math.Pi*440*float64(i)/float64(sr)) * 8000)
		binary.LittleEndian.PutUint16(data[44+i*4:], uint16(v))
		binary.LittleEndian.PutUint16(data[44+i*4+2:], uint16(v))
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeTestMP3 crafts a CBR 128kbps/44.1kHz stream: empty ID3v2 + N frames.
// Frame size = 144*128000/44100 = 417 bytes (no padding).
func writeTestMP3(t *testing.T, path string, frames int) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	f.Write([]byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, 0}) // empty tag
	frame := make([]byte, 417)
	frame[0], frame[1], frame[2], frame[3] = 0xFF, 0xFB, 0x90, 0x00
	for i := 0; i < frames; i++ {
		if _, err := f.Write(frame); err != nil {
			t.Fatal(err)
		}
	}
}

func writeTestFLAC(t *testing.T, path string, rate uint32, samples uint64) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	f.WriteString("fLaC")
	f.Write([]byte{0x80 | 0, 0, 0, 34}) // last, STREAMINFO, 34 bytes
	body := make([]byte, 34)
	body[10] = byte(rate >> 12)
	body[11] = byte(rate >> 4)
	body[12] = byte((rate & 0xF) << 4)
	body[13] = byte(samples >> 32)
	body[14] = byte(samples >> 24)
	body[15] = byte(samples >> 16)
	body[16] = byte(samples >> 8)
	body[17] = byte(samples)
	if _, err := f.Write(body); err != nil {
		t.Fatal(err)
	}
}

func TestFastWAVExact(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.wav")
	writeTestWAV(t, p, 2.0)
	if got := FastDuration(p); got != 2*time.Second {
		t.Fatalf("want 2s got %v", got)
	}
}

func TestFastMP3CBR(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.mp3")
	writeTestMP3(t, p, 100) // 100*1152/44100 = 2.612s
	got := FastDuration(p)
	want := 100 * 1152.0 / 44100.0
	if math.Abs(got.Seconds()-want) > 0.1 {
		t.Fatalf("want ~%.3fs got %v", want, got)
	}
}

func TestFastFLACExact(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.flac")
	writeTestFLAC(t, p, 44100, 441000) // 10s
	if got := FastDuration(p); got != 10*time.Second {
		t.Fatalf("want 10s got %v", got)
	}
}

func TestFastGarbageZero(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"x.mp3", "x.wav", "x.flac", "x.ogg", "x.txt"} {
		p := filepath.Join(dir, f)
		if err := os.WriteFile(p, []byte("garbage-data-here"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := FastDuration(p); got != 0 {
			t.Fatalf("%s: want 0 got %v", f, got)
		}
	}
	if got := FastDuration(filepath.Join(dir, "missing.mp3")); got != 0 {
		t.Fatalf("missing: want 0 got %v", got)
	}
}

func BenchmarkFastMP3(b *testing.B) {
	dir := b.TempDir()
	p := filepath.Join(dir, "x.mp3")
	f, _ := os.Create(p)
	f.Write([]byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, 0})
	frame := make([]byte, 417)
	frame[0], frame[1], frame[2], frame[3] = 0xFF, 0xFB, 0x90, 0x00
	for i := 0; i < 500; i++ {
		f.Write(frame)
	}
	f.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FastDuration(p)
	}
}
