package server

import (
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func btoi(s []byte) (int, error) {
	n := 0
	for _, ch := range s {
		ch -= '0'
		if ch > 9 {
			return 0, errByteToInt
		}
		n = n*10 + int(ch)
	}
	return n, nil
}

func sliceCopy(s []byte) []byte {
	c := make([]byte, len(s)) //creating a copy of the slice to avoid data race condition
	copy(c, s)
	return c
}
