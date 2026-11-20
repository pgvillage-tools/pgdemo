package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Loading a table with COPY ... WITH (FREEZE), in text format.
//
// FREEZE writes the rows as already frozen (and marks the pages all-visible):
// no VACUUM pass is needed afterwards and the first queries on the new data
// are not slowed down. PostgreSQL allows it because each table is created in
// the same transaction that fills it. pgx's CopyFrom has no COPY options, so the
// rows are encoded here.
//
// One goroutine generates and encodes the rows while the server processes the
// previous ones (the order of the generated values, and so the data for a
// seed, does not change).

const copyChunk = 256 << 10 // bytes per chunk sent to the server

// CopyStats tells where the time of one table went.
type CopyStats struct {
	Total, Generating, Waiting time.Duration
}

func (s CopyStats) String() string {
	return fmt.Sprintf("%s (generating %s, waiting for the server %s)",
		s.Total.Round(100*time.Millisecond), s.Generating.Round(100*time.Millisecond), s.Waiting.Round(100*time.Millisecond))
}

// copyFreeze loads count rows; row(k) gives row k (1-based), progress(n) is
// called every 5,000 rows.
func copyFreeze(ctx context.Context, tx pgx.Tx, schema, table string, cols []string, count int,
	row func(k int) []any, progress func(n int)) (CopyStats, error) {
	start := time.Now()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	chunks := make(chan []byte, 8)
	genErr := make(chan error, 1)
	var st CopyStats
	go func() {
		defer close(chunks)
		buf := make([]byte, 0, copyChunk+4096)
		var gen, wait time.Duration
		send := func() bool {
			t0 := time.Now()
			select {
			case chunks <- buf:
			case <-ctx.Done():
				return false
			}
			wait += time.Since(t0)
			buf = make([]byte, 0, copyChunk+4096)
			return true
		}
		t0 := time.Now()
		for k := 1; k <= count; k++ {
			if k%5000 == 0 {
				if ctx.Err() != nil {
					genErr <- ctx.Err()
					return
				}
				progress(k)
			}
			vals := row(k)
			if len(vals) != len(cols) {
				genErr <- fmt.Errorf("%s: row %d has %d values for %d columns", table, k, len(vals), len(cols))
				return
			}
			for i, v := range vals {
				if i > 0 {
					buf = append(buf, '\t')
				}
				var err error
				if buf, err = appendCopyText(buf, v); err != nil {
					genErr <- fmt.Errorf("%s.%s: %w", table, cols[i], err)
					return
				}
			}
			buf = append(buf, '\n')
			if len(buf) >= copyChunk {
				gen += time.Since(t0)
				if !send() {
					genErr <- ctx.Err()
					return
				}
				t0 = time.Now()
			}
		}
		gen += time.Since(t0)
		if len(buf) > 0 && !send() {
			genErr <- ctx.Err()
			return
		}
		st.Generating, st.Waiting = gen, wait
		genErr <- nil
	}()

	q := make([]string, len(cols))
	for i, c := range cols {
		q[i] = pgx.Identifier{c}.Sanitize()
	}
	sql := fmt.Sprintf("COPY %s (%s) FROM STDIN WITH (FREEZE)", pgx.Identifier{schema, table}.Sanitize(), strings.Join(q, ", "))
	_, err := tx.Conn().PgConn().CopyFrom(ctx, &chunkReader{ch: chunks, genErr: genErr}, sql)
	cancel() // stops the generator if COPY failed
	if gerr := <-genErr; gerr != nil && err == nil {
		err = gerr
	}
	st.Total = time.Since(start)
	return st, err
}

// chunkReader feeds the generated chunks to COPY.
type chunkReader struct {
	ch     <-chan []byte
	genErr chan error
	cur    []byte
}

func (r *chunkReader) Read(p []byte) (int, error) {
	for len(r.cur) == 0 {
		c, ok := <-r.ch
		if !ok {
			// generator finished: peek at its result without consuming it for good
			err := <-r.genErr
			r.genErr <- err
			if err != nil {
				return 0, err
			}
			return 0, io.EOF
		}
		r.cur = c
	}
	n := copy(p, r.cur)
	r.cur = r.cur[n:]
	return n, nil
}

// appendCopyText appends v in COPY text format.
func appendCopyText(b []byte, v any) ([]byte, error) {
	switch x := v.(type) {
	case nil:
		return append(b, `\N`...), nil
	case string:
		return appendEscaped(b, x), nil
	case int:
		return strconv.AppendInt(b, int64(x), 10), nil
	case int64:
		return strconv.AppendInt(b, x, 10), nil
	case int32:
		return strconv.AppendInt(b, int64(x), 10), nil
	case int16:
		return strconv.AppendInt(b, int64(x), 10), nil
	case uint64:
		return strconv.AppendUint(b, x, 10), nil
	case float64:
		return strconv.AppendFloat(b, x, 'f', -1, 64), nil
	case float32:
		return strconv.AppendFloat(b, float64(x), 'f', -1, 32), nil
	case bool:
		if x {
			return append(b, 't'), nil
		}
		return append(b, 'f'), nil
	case time.Time:
		return x.AppendFormat(b, "2006-01-02 15:04:05.999999Z07:00"), nil
	case time.Duration:
		return append(strconv.AppendFloat(b, x.Seconds(), 'f', -1, 64), " seconds"...), nil
	case netip.Prefix:
		return append(b, x.String()...), nil
	case netip.Addr:
		return append(b, x.String()...), nil
	case []int:
		b = append(b, '{')
		for i, e := range x {
			if i > 0 {
				b = append(b, ',')
			}
			b = strconv.AppendInt(b, int64(e), 10)
		}
		return append(b, '}'), nil
	case []string:
		b = append(b, '{')
		for i, e := range x {
			if i > 0 {
				b = append(b, ',')
			}
			q := `"` + strings.ReplaceAll(strings.ReplaceAll(e, `\`, `\\`), `"`, `\"`) + `"`
			b = appendEscaped(b, q)
		}
		return append(b, '}'), nil
	case map[string]any, []any:
		j, err := json.Marshal(x)
		if err != nil {
			return b, err
		}
		return appendEscaped(b, string(j)), nil
	}
	return b, fmt.Errorf("value of type %T cannot be written", v)
}

func appendEscaped(b []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\':
			b = append(b, '\\', '\\')
		case '\n':
			b = append(b, '\\', 'n')
		case '\r':
			b = append(b, '\\', 'r')
		case '\t':
			b = append(b, '\\', 't')
		default:
			b = append(b, c)
		}
	}
	return b
}
