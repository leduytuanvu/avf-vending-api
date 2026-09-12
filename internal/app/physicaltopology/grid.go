package physicaltopology

import "fmt"

// AllSlotCodes returns canonical slot codes for a row-major grid (A1, A2, …).
func AllSlotCodes(rows, cols int) []string {
	if rows < 1 {
		rows = 1
	}
	if cols < 1 {
		cols = 1
	}
	out := make([]string, 0, rows*cols)
	for r := 0; r < rows; r++ {
		rowChar := rune('A' + r)
		for c := 1; c <= cols; c++ {
			out = append(out, fmt.Sprintf("%c%d", rowChar, c))
		}
	}
	return out
}

// SlotIndexFromCode returns 1-based linear index for row-major grid (A1=1 on 10 cols).
func SlotIndexFromCode(slotCode string, cols int) int32 {
	if cols < 1 {
		cols = 1
	}
	trimmed := slotCode
	if len(trimmed) < 2 {
		return 0
	}
	row := int(trimmed[0] - 'A')
	col := 0
	for i := 1; i < len(trimmed); i++ {
		if trimmed[i] < '0' || trimmed[i] > '9' {
			return 0
		}
		col = col*10 + int(trimmed[i]-'0')
	}
	if row < 0 || col < 1 {
		return 0
	}
	return int32(row*cols + col)
}
