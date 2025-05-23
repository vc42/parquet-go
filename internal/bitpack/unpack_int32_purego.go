//go:build purego || !amd64

package bitpack

import (
	"github.com/vc42/parquet-go/internal/unsafecast"
)

func unpackInt32(dst []int32, src []byte, bitWidth uint) {
	bits := unsafecast.BytesToUint32(src)
	bitMask := uint32(1<<bitWidth) - 1
	bitOffset := uint(0)

	for n := range dst {
		i := bitOffset / 32
		j := bitOffset % 32
		d := (bits[i] & (bitMask << j)) >> j
		if j+bitWidth > 32 {
			k := 32 - j
			d |= (bits[i+1] & (bitMask >> k)) << k
		}
		dst[n] = int32(d)
		bitOffset += bitWidth
	}
}
