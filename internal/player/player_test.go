package player

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeWAV(t *testing.T, path string, secs float64) {
	t.Helper()
	sr := 44100
	n := int(float64(sr) * secs)
	data := make([]byte, 44+n*4)
	copy(data[0:], []byte("RIFF"))
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
		v := int16(math.Sin(2*math.Pi*440*float64(i)/float64(sr)) * 12000)
		binary.LittleEndian.PutUint16(data[44+i*4:], uint16(v))
		binary.LittleEndian.PutUint16(data[44+i*4+2:], uint16(v))
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadIsInstant(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tone.wav")
	writeWAV(t, p, 30.0) // 30s file; streaming Load must not decode it
	pl := New()
	start := time.Now()
	if err := pl.Load(p, 30*44100); err != nil {
		t.Fatal(err)
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Fatalf("Load took %v, must be instant (streaming)", el)
	}
	defer pl.Close()
	if got := pl.Duration(); got != 30*44100 {
		t.Fatalf("duration want %d got %d", 30*44100, got)
	}
}

func TestSeekJumps(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tone.wav")
	writeWAV(t, p, 10.0)
	pl := New()
	defer pl.Close()
	if err := pl.Load(p, 10*44100); err != nil {
		t.Fatal(err)
	}
	pl.SeekBy(5 * 44100)
	if got := pl.Position(); got < 5*44100-3000 || got > 5*44100+3000 {
		t.Fatalf("after +5s seek want ~%d got %d", 5*44100, got)
	}
	pl.SeekBy(-3 * 44100)
	if got := pl.Position(); got < 2*44100-3000 || got > 2*44100+3000 {
		t.Fatalf("after -3s seek want ~%d got %d", 2*44100, got)
	}
	pl.SeekBy(-100 * 44100) // clamp to 0
	if got := pl.Position(); got != 0 {
		t.Fatalf("clamp low want 0 got %d", got)
	}
	pl.SeekBy(1000 * 44100) // clamp to length
	if got := pl.Position(); got != 10*44100 {
		t.Fatalf("clamp high want %d got %d", 10*44100, got)
	}
}

func TestPlayAdvances(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tone.wav")
	writeWAV(t, p, 5.0)
	pl := New()
	defer pl.Close()
	if err := pl.Load(p, 5*44100); err != nil {
		t.Fatal(err)
	}
	if err := pl.Play(); err != nil {
		t.Skipf("no audio device: %v", err)
	}
	time.Sleep(400 * time.Millisecond)
	pos := pl.Position()
	if pos < 5000 || pos > 60000 {
		t.Fatalf("position should advance in realtime, got %d", pos)
	}
}

func TestLoadMissing(t *testing.T) {
	pl := New()
	if err := pl.Load("/nonexistent/x.mp3", 0); err == nil {
		t.Fatal("want error")
	}
	if err := pl.Play(); err == nil {
		t.Fatal("play without load should fail")
	}
}
