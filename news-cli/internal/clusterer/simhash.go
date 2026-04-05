package clusterer

import (
	"crypto/md5"
	"encoding/binary"
	"math/bits"
	"strings"
	"unicode"
)

type SimHash struct{}

func NewSimHash() *SimHash {
	return &SimHash{}
}

func (s *SimHash) Fingerprint(text string) uint64 {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})

	if len(words) == 0 {
		return 0
	}

	v := make([]int, 64)
	for _, word := range words {
		if len(word) < 3 {
			continue
		}
		
		h := md5.Sum([]byte(word))
		hash := binary.BigEndian.Uint64(h[:8])

		for i := 0; i < 64; i++ {
			if (hash>>uint(i))&1 == 1 {
				v[i]++
			} else {
				v[i]--
			}
		}
	}

	var fingerprint uint64
	for i := 0; i < 64; i++ {
		if v[i] > 0 {
			fingerprint |= (1 << uint(i))
		}
	}
	return fingerprint
}

func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

func (s *SimHash) IsSimilar(text1, text2 string, threshold int) bool {
	f1 := s.Fingerprint(text1)
	f2 := s.Fingerprint(text2)
	return HammingDistance(f1, f2) <= threshold
}
