package tdb

import (
	"bytes"
	"encoding/binary"
	"math/rand"

	"github.com/dustin/go-humanize"
	"github.com/golang/snappy"

	"testing"
)

const numPeriods = 365 * 24

func TestSnappyCompression(t *testing.T) {
	for i := 0.01; i <= 1; i += 0.01 {
		buf := bytes.NewBuffer(make([]byte, 0, numPeriods*8))
		for j := 0; j < numPeriods; j++ {
			val := rand.Float64()
			if val > i {
				val = 0
			}
			binary.Write(buf, binary.BigEndian, val)
		}
		b := buf.Bytes()
		compressed := snappy.Encode(make([]byte, len(b)), b)
		log.Debugf("Fill Rate: %f\tUncompressed: %v\tCompressed: %v\tRatio: %f", i, humanize.Comma(int64(len(b))), humanize.Comma(int64(len(compressed))), float64(len(compressed))/float64(len(b)))
	}
}
