package player

import (
	"testing"

	"github.com/gopxl/beep/v2"
)

func loopedSilence(n int) beep.Streamer {
	buf := beep.NewBuffer(beep.Format{SampleRate: 44100, NumChannels: 2, Precision: 2})
	buf.Append(beep.Silence(n))
	return beep.Iterate(func() beep.Streamer { return buf.Streamer(0, n) })
}

func BenchmarkTapStream512(b *testing.B) {
	tap := newTap(loopedSilence(44100))
	samples := make([][2]float64, 512)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tap.Stream(samples)
	}
}
