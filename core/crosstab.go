package core

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/getlantern/bytemap"
)

func Crosstab(source FlatRowSource) FlatRowSource {
	return &crosstabber{
		flatRowTransform{source},
	}
}

type crosstabber struct {
	flatRowTransform
}

func (c *crosstabber) Iterate(ctx context.Context, onFields OnFields, onRow OnFlatRow) error {
	var inFields Fields
	var inRows crosstabRows
	uniqueCTs := make(map[string]interface{})

	err := c.source.Iterate(ctx, func(fields Fields) error {
		inFields = fields
		return nil
	}, func(row *FlatRow) (bool, error) {
		_ct, key := row.Key.Split("_crosstab")
		ct := _ct.Get("_crosstab").(string)
		inRows = append(inRows, &crosstabRow{row, ct, key})
		uniqueCTs[ct] = nil
		return proceed()
	})

	numInFields := len(inFields)
	if err != ErrDeadlineExceeded {
		deadline, hasDeadline := ctx.Deadline()

		numCTs := len(uniqueCTs)
		// Build crosstab fields
		cts := make([]string, 0, numCTs)
		for ct := range uniqueCTs {
			cts = append(cts, ct)
		}
		sort.Strings(cts)
		outFields := make(Fields, 0, (numCTs+1)*numInFields)
		ctis := make(map[string]int, 1000)
		for cti, ct := range cts {
			ctis[ct] = cti
			ct = strings.Replace(ct, " ", "_", -1)
			for _, field := range inFields {
				outFields = append(outFields, NewField(fmt.Sprintf("%v_%v", ct, field.Name), field.Expr))
			}
		}
		for _, field := range inFields {
			outFields = append(outFields, NewField(fmt.Sprintf("total_%v", field.Name), field.Expr))
		}

		// Let caller know about fields
		err = onFields(outFields)
		if err != nil {
			return err
		}

		// Sort input rows and build output rows
		sort.Sort(inRows)
		numFields := len(outFields)
		var priorKey bytemap.ByteMap
		var priorTS int64
		var currentRow *FlatRow
		rowNeedsSubmission := false

		for _, row := range inRows {
			if hasDeadline && time.Now().After(deadline) {
				return ErrDeadlineExceeded
			}

			newRow := currentRow == nil || !bytes.Equal(row.key, priorKey) || priorTS > row.row.TS
			if newRow {
				if rowNeedsSubmission {
					more, onRowErr := onRow(currentRow)
					if onRowErr != nil {
						return onRowErr
					}
					if !more {
						break
					}
				}
				currentRow = &FlatRow{
					TS:     row.row.TS,
					Key:    row.key,
					Values: make([]float64, numFields),
					fields: outFields,
				}
			}
			cti := ctis[row.ct]
			for i, value := range row.row.Values {
				fieldIdx := cti*numInFields + i
				totalIdx := numCTs*numInFields + i
				currentRow.Values[fieldIdx] = value
				currentRow.Values[totalIdx] += value
			}
			rowNeedsSubmission = true
		}

		if rowNeedsSubmission {
			_, onRowErr := onRow(currentRow)
			return onRowErr
		}
	}

	return err
}

func (c *crosstabber) String() string {
	return fmt.Sprintf("crosstab")
}

type crosstabRow struct {
	row *FlatRow
	ct  string
	key bytemap.ByteMap
}

type crosstabRows []*crosstabRow

func (r crosstabRows) Len() int      { return len(r) }
func (r crosstabRows) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r crosstabRows) Less(i, j int) bool {
	a, b := r[i], r[j]

	// Sort primarily by key
	kc := bytes.Compare(a.key, b.key)
	if kc < 0 {
		return true
	}
	if kc > 0 {
		return false
	}

	// Then sort by TS (descending)
	if a.row.TS > b.row.TS {
		return true
	}
	if a.row.TS < b.row.TS {
		return false
	}

	// Lastly sort by ct
	return a.ct < b.ct
}