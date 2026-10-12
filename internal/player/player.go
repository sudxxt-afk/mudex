// Package player streams mp3/flac/wav/ogg (pure Go, seekable decoders) and
// falls back to a streaming ffmpeg pipe for m4a/opus/aac/wma.
// Nothing is fully decoded upfront: Load only parses headers, so playback
// starts in milliseconds with minimal CPU/RAM. Output via miniaudio (malgo).
package player

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gen2brain/malgo"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
)

const outRate = 44100

// source is a seekable PCM stream in its native sample rate.
type source interface {
	beep.Streamer
	SeekSrc(p int) error // seek to source-rate frame
	SrcRate() int
	Close() error
}

// ---------- native beep source ----------

type nativeSource struct {
	dec  beep.StreamSeekCloser
	file *os.File // non-nil when we own it (wav/flac); mp3/vorbis own their rc
	rate int
}

func (s *nativeSource) Stream(out [][2]float64) (int, bool) { return s.dec.Stream(out) }
func (s *nativeSource) SeekSrc(p int) error                 { return s.dec.Seek(p) }
func (s *nativeSource) SrcRate() int                        { return s.rate }
func (s *nativeSource) Err() error {
	if e, ok := s.dec.(interface{ Err() error }); ok {
		return e.Err()
	}
	return nil
}

func (s *nativeSource) Close() error {
	err := s.dec.Close()
	if s.file != nil {
		if ferr := s.file.Close(); err == nil {
			err = ferr
		}
	}
	return err
}

func openNative(path string) (*nativeSource, beep.Format, error) {
	ext := strings.ToLower(filepath.Ext(path))
	fh, err := os.Open(path)
	if err != nil {
		return nil, beep.Format{}, err
	}
	var (
		dec beep.StreamSeekCloser
		f   beep.Format
		own = true // we own fh unless decoder takes it
	)
	switch ext {
	case ".mp3":
		own = false
		dec, f, err = mp3.Decode(fh)
	case ".wav":
		dec, f, err = wav.Decode(fh)
	case ".flac":
		dec, f, err = flac.Decode(fh)
	case ".ogg", ".oga":
		own = false
		dec, f, err = vorbis.Decode(fh)
	default:
		_ = fh.Close()
		return nil, beep.Format{}, fmt.Errorf("unsupported")
	}
	if err != nil {
		_ = fh.Close()
		return nil, beep.Format{}, err
	}
	var file *os.File
	if own {
		file = fh
	}
	return &nativeSource{dec: dec, file: file, rate: int(f.SampleRate)}, f, nil
}

// ---------- streaming ffmpeg source (exotic formats) ----------

type ffmpegSource struct {
	path string
	cmd  *exec.Cmd
	out  io.ReadCloser
	left []byte // leftover raw bytes (<1 frame)
	held [][2]float64
	eof  bool
}

// pushBack re-queues already-read frames (used after open-time validation).
func (s *ffmpegSource) pushBack(f [2]float64) { s.held = append(s.held, f) }

func spawnFFmpeg(path string, seekSec float64) (*exec.Cmd, io.ReadCloser, error) {
	args := []string{"-v", "error"}
	if seekSec > 0.5/float64(outRate) {
		args = append(args, "-ss", fmt.Sprintf("%.3f", seekSec))
	}
	args = append(args, "-i", path, "-f", "f32le", "-ac", "2", "-ar", "44100", "-")
	cmd := exec.Command("ffmpeg", args...)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return cmd, pipe, nil
}

func openFFmpeg(path string) (*ffmpegSource, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("unsupported format %s (install ffmpeg for m4a/opus/aac/wma)", filepath.Ext(path))
	}
	cmd, pipe, err := spawnFFmpeg(path, 0)
	if err != nil {
		return nil, err
	}
	src := &ffmpegSource{path: path, cmd: cmd, out: pipe}
	// eager validation: missing/corrupt input yields instant EOF — fail fast
	// instead of "playing" silence
	probe := make([][2]float64, 1)
	if n, _ := src.Stream(probe); n == 0 {
		src.Close()
		return nil, fmt.Errorf("ffmpeg cannot decode %s", path)
	}
	// push the probed frame back so playback starts from 0
	src.pushBack(probe[0])
	return src, nil
}

func (s *ffmpegSource) SrcRate() int { return outRate }

func (s *ffmpegSource) Stream(out [][2]float64) (int, bool) {
	if len(s.held) > 0 {
		n := copy(out, s.held)
		s.held = append(s.held[:0], s.held[n:]...)
		return n, true
	}
	if s.eof {
		return 0, false
	}
	need := len(out) * 8 // 2ch x f32
	raw := make([]byte, 0, need)
	if len(s.left) > 0 {
		raw = append(raw, s.left...)
		s.left = nil
	}
	for len(raw) < need {
		chunk := make([]byte, need-len(raw))
		n, err := s.out.Read(chunk)
		if n > 0 {
			raw = append(raw, chunk[:n]...)
		}
		if err != nil {
			s.eof = true
			break
		}
	}
	frames := len(raw) / 8
	s.left = append(s.left[:0], raw[frames*8:]...)
	for i := 0; i < frames && i < len(out); i++ {
		l := math.Float32frombits(binary.LittleEndian.Uint32(raw[i*8:]))
		r := math.Float32frombits(binary.LittleEndian.Uint32(raw[i*8+4:]))
		if math.IsNaN(float64(l)) || math.IsInf(float64(l), 0) {
			l = 0
		}
		if math.IsNaN(float64(r)) || math.IsInf(float64(r), 0) {
			r = 0
		}
		out[i][0], out[i][1] = float64(l), float64(r)
	}
	if frames == 0 {
		return 0, false
	}
	return frames, true
}

func (s *ffmpegSource) SeekSrc(p int) error {
	s.kill()
	cmd, pipe, err := spawnFFmpeg(s.path, float64(p)/float64(outRate))
	if err != nil {
		return err
	}
	s.cmd, s.out, s.left, s.held, s.eof = cmd, pipe, nil, nil, false
	return nil
}

func (s *ffmpegSource) Err() error { return nil }

func (s *ffmpegSource) kill() {
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_ = s.cmd.Wait()
		s.cmd = nil
	}
	if s.out != nil {
		_ = s.out.Close()
		s.out = nil
	}
}

func (s *ffmpegSource) Close() error {
	s.kill()
	return nil
}

// ---------- player ----------

// Player holds current track state. All chain swaps happen under p.mu so the
// audio callback (holding RLock) never observes a torn chain.
type Player struct {
	mu      sync.RWMutex
	src     source
	res     *beep.Resampler // nil when src is already outRate (ffmpeg)
	tap     *tapStreamer
	ctrl    *beep.Ctrl
	vol     *effects.Volume
	out     *malgoOut
	playing bool
	loaded  string
	length  int // known length in output frames, 0 = unknown
	genBase int // output frames before current tap generation
	volume  float64
	baseVol float64
}

// New returns an empty player.
func New() *Player {
	return &Player{baseVol: -1.0}
}

// Loaded returns current path or "".
func (p *Player) Loaded() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.loaded
}

// IsPlaying reports playback state.
func (p *Player) IsPlaying() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.playing && p.ctrl != nil && !p.ctrl.Paused
}

// Duration returns known track length in frames (0 = unknown).
func (p *Player) Duration() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.length
}

// SampleRate returns output rate.
func (p *Player) SampleRate() int { return outRate }

// Frames converts a duration to output frames.
func (p *Player) Frames(d time.Duration) int { return int(d.Seconds() * outRate) }

// Position returns played frames.
func (p *Player) Position() int {
	p.mu.RLock()
	base := p.genBase
	tap := p.tap
	p.mu.RUnlock()
	if tap == nil {
		return base
	}
	return base + int(atomic.LoadInt64(&tap.count))
}

// Volume returns dB offset.
func (p *Player) Volume() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.volume
}

// Load opens path for streaming (headers only — fast) and prepares playback.
// lengthFrames is the known duration in output frames, or 0 if unknown.
func (p *Player) Load(path string, lengthFrames int) error {
	src, rate, err := openSource(path)
	if err != nil {
		return err
	}
	tap, ctrl, vol, res := buildChain(src, rate, p.volume+p.baseVol)

	p.mu.Lock()
	if p.src != nil {
		_ = p.src.Close()
	}
	p.src = src
	p.res = res
	p.tap = tap
	p.ctrl = ctrl
	p.vol = vol
	p.length = lengthFrames
	p.loaded = path
	p.playing = false
	p.genBase = 0
	p.mu.Unlock()
	return nil
}

// buildChain wraps src (native rate) into resampler->tap->ctrl->vol.
func buildChain(src source, rate int, volDB float64) (*tapStreamer, *beep.Ctrl, *effects.Volume, *beep.Resampler) {
	var top beep.Streamer = src
	var res *beep.Resampler
	if rate != outRate {
		res = beep.ResampleRatio(3, float64(outRate)/float64(rate), src)
		top = res
	}
	tap := newTap(top)
	ctrl := &beep.Ctrl{Streamer: tap, Paused: true}
	vol := &effects.Volume{Streamer: ctrl, Base: 2, Volume: volDB, Silent: false}
	return tap, ctrl, vol, res
}

func openSource(path string) (source, int, error) {
	if src, f, err := openNative(path); err == nil {
		return src, int(f.SampleRate), nil
	}
	src, err := openFFmpeg(path)
	if err != nil {
		return nil, 0, err
	}
	return src, outRate, nil
}

// ensureOut lazily opens the miniaudio playback device.
// MUST be called without holding p.mu (see deadlock note on first version).
func (p *Player) ensureOut() error {
	p.mu.RLock()
	if p.out != nil {
		p.mu.RUnlock()
		return nil
	}
	p.mu.RUnlock()
	o := &malgoOut{player: p}
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(string) {})
	if err != nil {
		return fmt.Errorf("audio: %w", err)
	}
	o.ctx = ctx
	devCfg := malgo.DefaultDeviceConfig(malgo.Playback)
	devCfg.Playback.Format = malgo.FormatF32
	devCfg.Playback.Channels = 2
	devCfg.SampleRate = outRate
	devCfg.Alsa.NoMMap = 1
	callbacks := malgo.DeviceCallbacks{Data: o.dataCallback}
	dev, err := malgo.InitDevice(ctx.Context, devCfg, callbacks)
	if err != nil {
		ctx.Uninit()
		return fmt.Errorf("audio device: %w", err)
	}
	o.dev = dev
	if err := dev.Start(); err != nil {
		dev.Uninit()
		ctx.Uninit()
		return fmt.Errorf("audio start: %w", err)
	}
	p.mu.Lock()
	if p.out == nil {
		p.out = o
	} else {
		dev.Uninit()
		ctx.Uninit()
	}
	p.mu.Unlock()
	return nil
}

// Play starts or resumes playback.
func (p *Player) Play() error {
	p.mu.RLock()
	empty := p.vol == nil
	p.mu.RUnlock()
	if empty {
		return fmt.Errorf("nothing loaded")
	}
	if err := p.ensureOut(); err != nil {
		return err
	}
	p.mu.Lock()
	p.playing = true
	if p.ctrl != nil {
		p.ctrl.Paused = false
	}
	p.mu.Unlock()
	return nil
}

// Pause suspends playback (device keeps running with silence).
func (p *Player) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ctrl != nil {
		p.ctrl.Paused = true
	}
	p.playing = false
}

// Toggle flips play/pause.
func (p *Player) Toggle() error {
	if p.IsPlaying() {
		p.Pause()
		return nil
	}
	return p.Play()
}

// Close stops playback and frees resources. Device teardown happens outside
// the lock: Uninit joins callback threads that need p.mu.RLock.
func (p *Player) Close() {
	p.mu.Lock()
	if p.ctrl != nil {
		p.ctrl.Paused = true
	}
	p.playing = false
	out := p.out
	p.out = nil
	src := p.src
	p.src = nil
	p.mu.Unlock()
	if out != nil {
		out.dev.Uninit()
		out.ctx.Uninit()
	}
	if src != nil {
		_ = src.Close()
	}
}

// SeekBy moves position by delta output frames (negative allowed).
func (p *Player) SeekBy(delta int) {
	p.mu.RLock()
	tap := p.tap
	length := p.length
	base := p.genBase
	p.mu.RUnlock()
	if tap == nil {
		return
	}
	target := base + int(atomic.LoadInt64(&tap.count)) + delta
	if target < 0 {
		target = 0
	}
	if length > 0 && target > length {
		target = length
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.src == nil || p.tap == nil {
		return
	}
	rate := p.src.SrcRate()
	if err := p.src.SeekSrc(target * rate / outRate); err != nil {
		return
	}
	// drop resampler history to avoid post-seek smear
	if _, ok := p.src.(*ffmpegSource); !ok && rate != outRate {
		p.res = beep.ResampleRatio(3, float64(outRate)/float64(rate), p.src)
		p.tap.retarget(p.res)
	} else {
		p.tap.retarget(p.src)
	}
	p.tap.reset()
	p.genBase = target
}

// VolumeAdd adjusts volume in dB steps.
func (p *Player) VolumeAdd(db float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.volume += db
	if p.volume > 6 {
		p.volume = 6
	}
	if p.volume < -10 {
		p.volume = -10
	}
	if p.vol != nil {
		p.vol.Volume = p.volume + p.baseVol
	}
}

// SpectrumPCM returns last n mono samples in [-1,1] for visualizer.
func (p *Player) SpectrumPCM(n int) []float64 {
	p.mu.RLock()
	tap := p.tap
	p.mu.RUnlock()
	if tap == nil {
		return make([]float64, n)
	}
	return tap.lastMono(n)
}

// Finished reports natural end of stream.
func (p *Player) Finished() bool {
	p.mu.RLock()
	tap := p.tap
	playing := p.playing && p.ctrl != nil && !p.ctrl.Paused
	p.mu.RUnlock()
	if tap == nil || !playing {
		return false
	}
	return tap.finished()
}

// --- miniaudio output ---

type malgoOut struct {
	player *Player
	ctx    *malgo.AllocatedContext
	dev    *malgo.Device
}

func (o *malgoOut) dataCallback(pOutput, _ []byte, frameCount uint32) {
	p := o.player
	p.mu.RLock()
	vol := p.vol
	paused := p.ctrl == nil || p.ctrl.Paused || !p.playing
	p.mu.RUnlock()
	want := int(frameCount) * 2
	if paused || vol == nil {
		for i := 0; i < want && (i+1)*4 <= len(pOutput); i++ {
			binary.LittleEndian.PutUint32(pOutput[i*4:], 0)
		}
		return
	}
	buf := make([][2]float64, frameCount)
	n, _ := vol.Stream(buf)
	for i := 0; i < int(frameCount); i++ {
		var l, r float32
		if i < n {
			l = float32(buf[i][0])
			r = float32(buf[i][1])
		}
		if (i*2+2)*4 <= len(pOutput) {
			binary.LittleEndian.PutUint32(pOutput[i*8:], math.Float32bits(l))
			binary.LittleEndian.PutUint32(pOutput[i*8+4:], math.Float32bits(r))
		}
	}
}

// --- tap streamer: counts frames + keeps ring for visualizer ---

type tapStreamer struct {
	mu   sync.Mutex
	src  beep.Streamer
	head int // ring write position (circular)
	ring []float64

	count int64 // output frames streamed in current generation
	done  int32 // 1 when src returned EOF
}

func newTap(src beep.Streamer) *tapStreamer {
	return &tapStreamer{src: src, ring: make([]float64, outRate*2)}
}

// retarget swaps the upstream (after seek). Caller must hold p.mu.
func (t *tapStreamer) retarget(src beep.Streamer) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.src = src
}

// reset starts a new generation (after seek/load).
func (t *tapStreamer) reset() {
	atomic.StoreInt64(&t.count, 0)
	atomic.StoreInt32(&t.done, 0)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.head = 0
	for i := range t.ring {
		t.ring[i] = 0
	}
}

func (t *tapStreamer) finished() bool { return atomic.LoadInt32(&t.done) == 1 }

func (t *tapStreamer) Stream(samples [][2]float64) (int, bool) {
	t.mu.Lock()
	n, ok := t.src.Stream(samples)
	for i := 0; i < n; i++ {
		t.ring[t.head] = (samples[i][0] + samples[i][1]) / 2
		t.head++
		if t.head >= len(t.ring) {
			t.head = 0
		}
	}
	t.mu.Unlock()
	atomic.AddInt64(&t.count, int64(n))
	if n == 0 && !ok {
		atomic.StoreInt32(&t.done, 1)
	}
	return n, ok
}

func (t *tapStreamer) Err() error {
	if e, ok := t.src.(interface{ Err() error }); ok {
		return e.Err()
	}
	return nil
}

func (t *tapStreamer) lastMono(n int) []float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if n > len(t.ring) {
		n = len(t.ring)
	}
	out := make([]float64, n)
	start := t.head - n
	if start < 0 {
		start += len(t.ring)
	}
	for i := 0; i < n; i++ {
		out[i] = t.ring[(start+i)%len(t.ring)]
	}
	return out
}
