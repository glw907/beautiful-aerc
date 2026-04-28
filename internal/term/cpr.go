// SPDX-License-Identifier: MIT

package term

import (
	"bufio"
	"errors"
	"io"
	"strconv"
)

var errCPRParse = errors.New("term: failed to parse CPR response")

// parseCPR reads bytes from r until it consumes a complete
// Cursor-Position-Report sequence "ESC [ <row> ; <col> R" and returns
// the (row, col) pair. Bytes preceding the ESC are skipped. Bytes
// following 'R' are left in the reader if r supports it; otherwise they
// are discarded (we only parse the first complete sequence).
func parseCPR(r io.Reader) (row, col int, err error) {
	br := bufio.NewReader(r)

	// Skip until ESC.
	for {
		b, e := br.ReadByte()
		if e != nil {
			return 0, 0, errCPRParse
		}
		if b == 0x1b {
			break
		}
	}
	// Expect '['.
	b, e := br.ReadByte()
	if e != nil || b != '[' {
		return 0, 0, errCPRParse
	}
	rowStr, e := readDigits(br)
	if e != nil || rowStr == "" {
		return 0, 0, errCPRParse
	}
	b, e = br.ReadByte()
	if e != nil || b != ';' {
		return 0, 0, errCPRParse
	}
	colStr, e := readDigits(br)
	if e != nil || colStr == "" {
		return 0, 0, errCPRParse
	}
	b, e = br.ReadByte()
	if e != nil || b != 'R' {
		return 0, 0, errCPRParse
	}
	row, _ = strconv.Atoi(rowStr)
	col, _ = strconv.Atoi(colStr)
	return row, col, nil
}

func readDigits(br *bufio.Reader) (string, error) {
	var s []byte
	for {
		b, err := br.ReadByte()
		if err != nil {
			return "", err
		}
		if b < '0' || b > '9' {
			_ = br.UnreadByte()
			return string(s), nil
		}
		s = append(s, b)
	}
}
