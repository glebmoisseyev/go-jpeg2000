package jpeg2000

import (
	"bytes"
	"sync"
	"testing"
)

// TestDecodeHTConcurrentStable decodes HTJ2K images on several goroutines at
// once. The HT block decoder is pooled, and its output buffer belongs to the
// decoder: a caller that returns the decoder to the pool before it has read
// the coefficients lets another goroutine overwrite them. Run with -race.
func TestDecodeHTConcurrentStable(t *testing.T) {
	const w, h, workers, rounds = 256, 256, 8, 12
	o := DefaultOptions()
	o.Format = FormatJ2K
	o.HighThroughput = true
	o.NumResolutions = 4
	o.ProgressionOrder = RPCL

	type frame struct {
		vals []int16
		code []byte
	}
	frames := make([]frame, workers)
	for i := range frames {
		vals := make([]int16, w*h)
		seed := uint32(i + 1)
		for j := range vals {
			seed = seed*1664525 + 1013904223
			vals[j] = int16(seed>>16)/8 + int16(i*37)
		}
		var buf bytes.Buffer
		if err := EncodeInt16(&buf, &Int16Image{Width: w, Height: h, Components: [][]int16{vals}}, o); err != nil {
			t.Fatal(err)
		}
		frames[i] = frame{vals: vals, code: buf.Bytes()}
	}

	var wg sync.WaitGroup
	errs := make(chan string, workers)
	for i := range frames {
		wg.Add(1)
		go func(f frame) {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				img, err := DecodeInt16(bytes.NewReader(f.code))
				if err != nil {
					errs <- err.Error()
					return
				}
				got := img.Components[0]
				for j := range f.vals {
					if got[j] != f.vals[j] {
						errs <- "decoded samples differ from the input"
						return
					}
				}
			}
		}(frames[i])
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}
