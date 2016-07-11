package tdb

import (
	. "github.com/getlantern/tdb/expr"
	"time"

	"github.com/stretchr/testify/assert"
	"testing"
)

func TestToValues(t *testing.T) {
	epoch := time.Date(2015, 5, 6, 7, 8, 9, 10, time.UTC)
	res := time.Minute
	b := &bucket{
		start: epoch.Add(10 * res),
		vals:  []Accumulator{CONST(6).Accumulator()},
		prev: &bucket{
			start: epoch.Add(7 * res),
			vals:  []Accumulator{CONST(5).Accumulator()},
			prev: &bucket{
				start: epoch.Add(5 * res),
				vals:  []Accumulator{CONST(4).Accumulator()},
			},
		},
	}

	vals := b.toValues(res)
	if !assert.Len(t, vals, 1) {
		return
	}
	vs := vals[0]
	if !assert.Len(t, vs, 3) {
		return
	}

	checkValue := func(idx int, expectedOffset time.Duration, expectedValue float64) {
		assert.Equal(t, epoch.Add(expectedOffset), sequence(vs[idx]).start().In(time.UTC))
		assert.EqualValues(t, expectedValue, sequence(vs[idx]).valueAt(0))
	}

	checkValue(0, 10*res, 6)
	checkValue(1, 7*res, 5)
	checkValue(2, 5*res, 4)
}

func TestSequence(t *testing.T) {
	epoch := time.Date(2015, 5, 6, 7, 8, 9, 10, time.UTC)
	res := time.Minute

	checkWithTruncation := func(retainPeriods int) {
		t.Logf("Retention periods: %d", retainPeriods)
		retentionPeriod := res * time.Duration(retainPeriods)
		trunc := func(vals []float64, ignoreTrailingIndex int) []float64 {
			if len(vals) > retainPeriods {
				vals = vals[:retainPeriods]
				if len(vals)-1 == ignoreTrailingIndex {
					// Remove trailing zero to deal with append deep
					vals = vals[:retainPeriods-1]
				}
			}
			return vals
		}

		start := epoch
		var seq sequence

		doIt := func(ts time.Time, value float64, expected []float64) {
			if ts.After(start) {
				start = ts
			}
			truncateBefore := start.Add(-1 * retentionPeriod)
			seq = seq.plus(newTSValue(ts, value), res, truncateBefore)
			checkValues(t, seq, trunc(expected, 4))
		}

		// Set something on an empty sequence
		doIt(epoch, 2, []float64{2})

		// Prepend
		doIt(epoch.Add(2*res), 1, []float64{1, 0, 2})

		// Append
		doIt(epoch.Add(-1*res), 3, []float64{1, 0, 2, 3})

		// Append deep
		doIt(epoch.Add(-3*res), 4, []float64{1, 0, 2, 3, 0, 4})

		// Update value
		doIt(epoch, 5, []float64{1, 0, 5, 3, 0, 4})
	}

	for i := 6; i >= 0; i-- {
		checkWithTruncation(i)
	}
}

func checkValues(t *testing.T, seq sequence, expected []float64) {
	if assert.Equal(t, len(expected), seq.numPeriods()) {
		for i, v := range expected {
			assert.EqualValues(t, v, seq.valueAt(i))
		}
	}
}
