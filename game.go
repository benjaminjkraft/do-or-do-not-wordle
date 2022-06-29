package main

import "fmt"

const (
	length  = 5
	letters = 26
)

type clue uint8

const (
	unknown clue = iota
	gray
	yellow
	green
)

func resultString(result [length]clue) string {
	b := make([]byte, length)
	for i, c := range result {
		switch c {
		case unknown:
			b[i] = ' '
		case gray:
			b[i] = '_'
		case yellow:
			b[i] = 'Y'
		case green:
			b[i] = 'G'
		}
	}
	return string(b)
}

// low 5 bits: bitmask of where the letter is
// high 3 bits: int3 of how many there are
type (
	charIndex uint8
	index     [letters]charIndex
)

func newIndex(word string) index {
	var ret index
	for i, c := range []byte(word) {
		ret[c-'a'] |= 1 << i
		ret[c-'a'] += 1 << length
	}
	return ret
}

func (ci charIndex) count() uint8 {
	return uint8(ci >> length)
}

func (ci charIndex) at(i int) bool {
	return ci&(1<<i) != 0
}

type state struct {
	target      string
	targetIndex index
	// high 5 bits: (letter that was green)+1, or 0 if none
	// low 26 bits: bitmask of letters that were yellow/gray
	clues [length]uint32
	// low 7 bits: max times yellow/green
	// high bit: 1 if exact (i.e. also had gray on that guess)
	counts [letters]uint8
}

func newState(target string) *state {
	if len(target) != length {
		panic(fmt.Sprintf("invalid target: len(%v) = %v", target, len(target)))
	}
	s := state{
		target:      target,
		targetIndex: newIndex(target),
	}

	return &s
}

func (s *state) guess(word string) (result [length]clue, won bool) {
	if len(word) != length {
		panic(fmt.Sprintf("invalid guess: len(%v) = %v", word, len(word)))
	}

	wordIndex := newIndex(word)
	for i, charIndex := range wordIndex {
		targetIndex := s.targetIndex[i]
		switch {
		case charIndex == 0:
			// didn't guess this character
			// (note this is actually a special case of the third case,
			// included for speed and clarity)
			continue
		case targetIndex == 0:
			// character not in target: this letter is gray
			// (note this is actually a special case of the last case, included
			// for speed and clarity)
			for j := 0; j < length; j++ {
				if charIndex.at(j) {
					result[j] = gray
					s.clues[j] |= 1 << i
				}
			}
			s.counts[i] = 0x80
		case charIndex.count() <= targetIndex.count():
			// guessed at most the right number of this letter: they'll all be
			// green/yellow.
			for j := 0; j < length; j++ {
				if charIndex.at(j) {
					if targetIndex.at(j) {
						result[j] = green
						s.clues[j] |= uint32(i+1) << (32 - length)
					} else {
						result[j] = yellow
						s.clues[j] |= 1 << i
					}
				}
			}
			s.counts[i] = charIndex.count()
		default:
			// guessed too many of this letter: correct positions are
			// green/yellow, then the first n are yellow to get the right
			// number, rest are gray.

			need := targetIndex.count()
			// first find green.
			for j := 0; j < length; j++ {
				if charIndex.at(j) && targetIndex.at(j) {
					result[j] = green
					s.clues[j] |= uint32(i+1) << (32 - length)
					need--
				}
			}
			// next find yellow/gray.
			for j := 0; j < length; j++ {
				if charIndex.at(j) && !targetIndex.at(j) {
					if need > 0 {
						result[j] = yellow
						s.clues[j] |= 1 << i
						need--
					} else {
						result[j] = gray
						s.clues[j] |= 1 << i
					}
				}
			}
			s.counts[i] = 0x80 | targetIndex.count()
		}
	}

	return result, result == [length]clue{green, green, green, green, green}
}

func (s *state) hardModeInfo(word string) (bool, string, byte, int) {
	if len(word) != length {
		panic(fmt.Sprintf("invalid guess: len(%v) = %v", word, len(word)))
	}

	wordIndex := newIndex(word)
	for i, n := range s.counts {
		max := n & 0x7f
		c := byte(i + 'a')
		if n&0x80 == 0x80 {
			// if prev guess has m copies of a letter, and k < m are
			// yellow/green, must use that letter exactly k times
			if wordIndex[i].count() != max {
				if max == 0 {
					return false, "can't use %c", c, -1
				} else {
					return false, "need to use %c exactly %d times", c, int(max)
				}
			}
		} else {
			// if prev guess has m copies of a letter, and all are
			// yellow/green, must use that letter at least m times
			if wordIndex[i].count() < max {
				return false, "need to use %c at least %d times", c, int(max)
			}
		}
	}

	for j, c := range []byte(word) {
		i := c - 'a'
		clue := s.clues[j]
		// if prev guess has green in a spot, must use that letter in that spot
		green := clue >> (32 - length)
		if green != 0 && i != byte(green-1) {
			return false, "need %c as letter %d", 'a' + byte(green-1), j + 1
		}
		// if prev guess has yellow or gray in a spot, must NOT use that letter
		// in that spot
		if clue&(1<<i) != 0 {
			return false, "can't use %c as letter %d", c, j + 1
		}
	}

	return true, "", 0, 0
}

func (s *state) hardModeProblem(word string) error {
	ok, str, b, n := s.hardModeInfo(word)
	switch {
	case ok:
		return nil
	case n == -1:
		return fmt.Errorf(str, b)
	default:
		return fmt.Errorf(str, b, n)
	}
}

func (s *state) hardModeOK(word string) bool {
	ok, _, _, _ := s.hardModeInfo(word)
	return ok
}
