package ansi

import "unicode/utf8"

// ConsumeEscape consumes an ANSI escape sequence starting at position i.
// Returns the position after the escape sequence.
func ConsumeEscape(s string, i int) int {
    if i >= len(s) || s[i] != 0x1b {
        if i+1 > len(s) {
            return len(s)
        }
        return i + 1
    }
    
    j := i + 1
    if j >= len(s) {
        return j
    }
    
    switch s[j] {
    case '[': // CSI
        j++
        for j < len(s) {
            c := s[j]
            if c >= 0x40 && c <= 0x7e {
                j++
                break
            }
            j++
        }
    case ']': // OSC
        j++
        for j < len(s) && s[j] != 0x07 {
            j++
        }
        if j < len(s) {
            j++
        }
    case 'P', 'X', '^', '_': // DCS, SOS, PM, APC
        j++
        for j < len(s) {
            if s[j] == 0x1b {
                j++
                break
            }
            j++
        }
    default:
        j++
    }
    
    if j <= i {
        return i + 1
    }
    return j
}

// Strip removes all ANSI escape sequences from the string.
func Strip(s string) string {
    var result []byte
    i := 0
    for i < len(s) {
        if s[i] == 0x1b {
            next := ConsumeEscape(s, i)
            i = next
            continue
        }
        result = append(result, s[i])
        i++
    }
    return string(result)
}

// VisualWidth returns the visual width of a string (rune count, excluding ANSI codes).
func VisualWidth(s string) int {
    plain := Strip(s)
    return utf8.RuneCountInString(plain)
}
