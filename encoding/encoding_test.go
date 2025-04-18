package encoding_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"math/bits"
	"math/rand"
	"testing"
	"time"

	"github.com/vc42/parquet-go/deprecated"
	"github.com/vc42/parquet-go/encoding"
	"github.com/vc42/parquet-go/encoding/bitpacked"
	"github.com/vc42/parquet-go/encoding/bytestreamsplit"
	"github.com/vc42/parquet-go/encoding/delta"
	"github.com/vc42/parquet-go/encoding/plain"
	"github.com/vc42/parquet-go/encoding/rle"
	"github.com/vc42/parquet-go/internal/unsafecast"
)

func repeatInt64(seq []int64, n int) []int64 {
	rep := make([]int64, len(seq)*n)
	for i := 0; i < n; i++ {
		copy(rep[i*len(seq):], seq)
	}
	return rep
}

var booleanTests = [...][]bool{
	{},
	{true},
	{false},
	{true, false, true, false, true, true, true, false, false, true},
	{ // repeating 32x
		true, true, true, true, true, true, true, true,
		true, true, true, true, true, true, true, true,
		true, true, true, true, true, true, true, true,
		true, true, true, true, true, true, true, true,
	},
	{ // repeating 33x
		true, true, true, true, true, true, true, true,
		true, true, true, true, true, true, true, true,
		true, true, true, true, true, true, true, true,
		true, true, true, true, true, true, true, true,
		true,
	},
	{ // alternating 15x
		false, true, false, true, false, true, false, true,
		false, true, false, true, false, true, false,
	},
	{ // alternating 16x
		false, true, false, true, false, true, false, true,
		false, true, false, true, false, true, false, true,
	},
}

var levelsTests = [...][]byte{
	{},
	{0},
	{1},
	{0, 1, 0, 2, 3, 4, 5, 6, math.MaxInt8, math.MaxInt8, 0},
	{ // repeating 24x
		42, 42, 42, 42, 42, 42, 42, 42,
		42, 42, 42, 42, 42, 42, 42, 42,
		42, 42, 42, 42, 42, 42, 42, 42,
	},
	{ // never repeating
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
		0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
		0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F,
	},
	{ // streaks of repeating values
		0, 0, 0, 0, 1, 1, 1, 1,
		2, 2, 2, 2, 3, 3, 3, 3,
		4, 4, 4, 4, 5, 5, 5, 5,
		6, 6, 6, 7, 7, 7, 8, 8,
		8, 9, 9, 9,
	},
}

var int32Tests = [...][]int32{
	{},
	{0},
	{1},
	{-1, 0, 1, 0, 2, 3, 4, 5, 6, math.MaxInt32, math.MaxInt32, 0},
	{ // repeating 24x
		42, 42, 42, 42, 42, 42, 42, 42,
		42, 42, 42, 42, 42, 42, 42, 42,
		42, 42, 42, 42, 42, 42, 42, 42,
	},
	{ // never repeating
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
		0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
		0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F,
	},
	{ // streaks of repeating values
		0, 0, 0, 0, 1, 1, 1, 1,
		2, 2, 2, 2, 3, 3, 3, 3,
		4, 4, 4, 4, 5, 5, 5, 5,
		6, 6, 6, 7, 7, 7, 8, 8,
		8, 9, 9, 9,
	},
	{ // a sequence that triggered a bug in the delta binary packed encoding
		24, 36, 47, 32, 29, 4, 9, 20, 2, 18,
	},
}

var int64Tests = [...][]int64{
	{},
	{0},
	{1},
	{-1, 0, 1, 0, 2, 3, 4, 5, 6, math.MaxInt64, math.MaxInt64, 0},
	{ // repeating 24x
		42, 42, 42, 42, 42, 42, 42, 42,
		42, 42, 42, 42, 42, 42, 42, 42,
		42, 42, 42, 42, 42, 42, 42, 42,
	},
	{ // never repeating
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
		0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
		0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F,
	},
	{ // streaks of repeating values
		0, 0, 0, 0, 1, 1, 1, 1,
		2, 2, 2, 2, 3, 3, 3, 3,
		4, 4, 4, 4, 5, 5, 5, 5,
		6, 6, 6, 7, 7, 7, 8, 8,
		8, 9, 9, 9,
	},
	{ // streaks of repeating values
		0, 0, 0, 0, 1, 1, 1, 1,
		2, 2, 2, 2, 3, 3, 3, 3,
		4, 4, 4, 4, 5, 5, 5, 5,
		6, 6, 6, 7, 7, 7, 8, 8,
		8, 9, 9, 9,
	},
	repeatInt64( // a sequence resulting in 64 bits words in the delta binary packed encoding
		[]int64{
			math.MinInt64, math.MaxInt64, math.MinInt64, math.MaxInt64,
			math.MinInt64, math.MaxInt64, math.MinInt64, math.MaxInt64,

			0, math.MaxInt64, math.MinInt64, math.MaxInt64,
			math.MinInt64, math.MaxInt64, math.MinInt64, math.MaxInt64,
		},
		5,
	),
}

var int96Tests = [...][]deprecated.Int96{
	{},
	{{0: 0}},
	{{0: 1}},
}

var floatTests = [...][]float32{
	{},
	{0},
	{1},
	{0, 1, 0, 1, 0, 2, 3, 4, 5, 6, math.MaxFloat32, math.MaxFloat32, 0},
	{-1, 0, 1, 0, 2, 3, 4, 5, 6, math.MaxFloat32, math.MaxFloat32, 0},
}

var doubleTests = [...][]float64{
	{},
	{0},
	{1},
	{-1, 0, 1, 0, 2, 3, 4, 5, 6, math.MaxFloat64, math.MaxFloat64, 0},
}

var byteArrayTests = [...][][]byte{
	{},
	{[]byte("")},
	{[]byte("A"), []byte("B"), []byte("C")},
	{[]byte("hello world!"), bytes.Repeat([]byte("1234567890"), 100)},
}

var fixedLenByteArrayTests = [...]struct {
	size int
	data []byte
}{
	{size: 1, data: []byte("")},
	{size: 1, data: []byte("ABCDEFGH")},
	{size: 2, data: []byte("ABCDEFGH")},
	{size: 4, data: []byte("ABCDEFGH")},
	{size: 8, data: []byte("ABCDEFGH")},
	{size: 10, data: bytes.Repeat([]byte("123456789"), 100)},
	{size: 16, data: bytes.Repeat([]byte("1234567890"), 160)},
}

var encodings = [...]encoding.Encoding{
	new(plain.Encoding),
	new(rle.Encoding),
	new(bitpacked.Encoding),
	new(plain.DictionaryEncoding),
	new(rle.DictionaryEncoding),
	new(delta.BinaryPackedEncoding),
	new(delta.LengthByteArrayEncoding),
	new(delta.ByteArrayEncoding),
	new(bytestreamsplit.Encoding),
}

func TestEncoding(t *testing.T) {
	for _, encoding := range encodings {
		t.Run(encoding.String(), func(t *testing.T) { testEncoding(t, encoding) })
	}
}

func testEncoding(t *testing.T, e encoding.Encoding) {
	for _, test := range [...]struct {
		scenario string
		function func(*testing.T, encoding.Encoding)
	}{
		{
			scenario: "boolean",
			function: testBooleanEncoding,
		},

		{
			scenario: "levels",
			function: testLevelsEncoding,
		},

		{
			scenario: "int32",
			function: testInt32Encoding,
		},

		{
			scenario: "int64",
			function: testInt64Encoding,
		},

		{
			scenario: "int96",
			function: testInt96Encoding,
		},

		{
			scenario: "float",
			function: testFloatEncoding,
		},

		{
			scenario: "double",
			function: testDoubleEncoding,
		},

		{
			scenario: "byte array",
			function: testByteArrayEncoding,
		},

		{
			scenario: "fixed length byte array",
			function: testFixedLenByteArrayEncoding,
		},
	} {
		t.Run(test.scenario, func(t *testing.T) { test.function(t, e) })
	}
}

func setBitWidth(enc encoding.Encoding, bitWidth int) {
	switch e := enc.(type) {
	case *rle.Encoding:
		e.BitWidth = bitWidth
	case *bitpacked.Encoding:
		e.BitWidth = bitWidth
	}
}

type encodingFunc func(encoding.Encoding, []byte, []byte) ([]byte, error)

func testBooleanEncoding(t *testing.T, e encoding.Encoding) {
	testCanEncodeBoolean(t, e)
	buffer := []byte{}
	values := []byte{}
	input := []byte{}
	setBitWidth(e, 1)

	for _, test := range booleanTests {
		t.Run("", func(t *testing.T) {
			var err error

			input = input[:0]
			count := 0
			for _, value := range test {
				input = plain.AppendBoolean(input, count, value)
				count++
			}

			buffer, err = e.EncodeBoolean(buffer, input)
			assertNoError(t, err)
			values, err = e.DecodeBoolean(values, buffer)
			assertNoError(t, err)
			assertBytesEqual(t, input, values)
		})
	}
}

func testLevelsEncoding(t *testing.T, e encoding.Encoding) {
	testCanEncodeLevels(t, e)
	buffer := []byte{}
	values := []byte{}

	for _, test := range levelsTests {
		setBitWidth(e, maxLenInt8(unsafecast.BytesToInt8(test)))

		t.Run("", func(t *testing.T) {
			var err error
			buffer, err = e.EncodeLevels(buffer, test)
			assertNoError(t, err)
			values, err = e.DecodeLevels(values, buffer)
			assertNoError(t, err)
			assertBytesEqual(t, test, values[:len(test)])
		})
	}
}

func testInt32Encoding(t *testing.T, e encoding.Encoding) {
	testCanEncodeInt32(t, e)
	buffer := []byte{}
	values := []byte{}

	for _, test := range int32Tests {
		setBitWidth(e, maxLenInt32(test))

		t.Run("", func(t *testing.T) {
			var err error
			var input = unsafecast.Int32ToBytes(test)
			buffer, err = e.EncodeInt32(buffer, input)
			assertNoError(t, err)
			values, err = e.DecodeInt32(values, buffer)
			assertNoError(t, err)
			assertBytesEqual(t, input, values)
		})
	}
}

func testInt64Encoding(t *testing.T, e encoding.Encoding) {
	testCanEncodeInt64(t, e)
	buffer := []byte{}
	values := []byte{}

	for _, test := range int64Tests {
		setBitWidth(e, maxLenInt64(test))

		t.Run("", func(t *testing.T) {
			var err error
			var input = unsafecast.Int64ToBytes(test)
			buffer, err = e.EncodeInt64(buffer, input)
			assertNoError(t, err)
			values, err = e.DecodeInt64(values, buffer)
			assertNoError(t, err)
			assertBytesEqual(t, input, values)
		})
	}
}

func testInt96Encoding(t *testing.T, e encoding.Encoding) {
	testCanEncodeInt96(t, e)
	buffer := []byte{}
	values := []byte{}

	for _, test := range int96Tests {
		t.Run("", func(t *testing.T) {
			var err error
			var input = deprecated.Int96ToBytes(test)
			buffer, err = e.EncodeInt96(buffer, input)
			assertNoError(t, err)
			values, err = e.DecodeInt96(values, buffer)
			assertNoError(t, err)
			assertBytesEqual(t, input, values)
		})
	}
}

func testFloatEncoding(t *testing.T, e encoding.Encoding) {
	testCanEncodeFloat(t, e)
	buffer := []byte{}
	values := []byte{}

	for _, test := range floatTests {
		t.Run("", func(t *testing.T) {
			var err error
			var input = unsafecast.Float32ToBytes(test)
			buffer, err = e.EncodeFloat(buffer, input)
			assertNoError(t, err)
			values, err = e.DecodeFloat(values, buffer)
			assertNoError(t, err)
			assertBytesEqual(t, input, values)
		})
	}
}

func testDoubleEncoding(t *testing.T, e encoding.Encoding) {
	testCanEncodeDouble(t, e)
	buffer := []byte{}
	values := []byte{}

	for _, test := range doubleTests {
		t.Run("", func(t *testing.T) {
			var err error
			var input = unsafecast.Float64ToBytes(test)
			buffer, err = e.EncodeDouble(buffer, input)
			assertNoError(t, err)
			values, err = e.DecodeDouble(values, buffer)
			assertNoError(t, err)
			assertBytesEqual(t, input, values)
		})
	}
}

func testByteArrayEncoding(t *testing.T, e encoding.Encoding) {
	testCanEncodeByteArray(t, e)
	buffer := []byte{}
	values := []byte{}
	byteArrays := []byte{}

	for _, test := range byteArrayTests {
		byteArrays = byteArrays[:0]

		for _, value := range test {
			byteArrays = plain.AppendByteArray(byteArrays, value)
		}

		t.Run("", func(t *testing.T) {
			var err error
			buffer, err = e.EncodeByteArray(buffer, byteArrays)
			assertNoError(t, err)
			values, err = e.DecodeByteArray(values, buffer)
			assertNoError(t, err)
			assertBytesEqual(t, byteArrays, values)
		})
	}
}

func testFixedLenByteArrayEncoding(t *testing.T, e encoding.Encoding) {
	testCanEncodeFixedLenByteArray(t, e)
	buffer := []byte{}
	values := []byte{}

	for _, test := range fixedLenByteArrayTests {
		t.Run("", func(t *testing.T) {
			var err error
			buffer, err = e.EncodeFixedLenByteArray(buffer, test.data, test.size)
			assertNoError(t, err)
			values, err = e.DecodeFixedLenByteArray(values, buffer, test.size)
			assertNoError(t, err)
			assertBytesEqual(t, test.data, values)
		})
	}
}

func testCanEncodeBoolean(t testing.TB, e encoding.Encoding) {
	testCanEncode(t, e, encoding.CanEncodeBoolean)
}

func testCanEncodeLevels(t testing.TB, e encoding.Encoding) {
	testCanEncode(t, e, encoding.CanEncodeLevels)
}

func testCanEncodeInt32(t testing.TB, e encoding.Encoding) {
	testCanEncode(t, e, encoding.CanEncodeInt32)
}

func testCanEncodeInt64(t testing.TB, e encoding.Encoding) {
	testCanEncode(t, e, encoding.CanEncodeInt64)
}

func testCanEncodeInt96(t testing.TB, e encoding.Encoding) {
	testCanEncode(t, e, encoding.CanEncodeInt96)
}

func testCanEncodeFloat(t testing.TB, e encoding.Encoding) {
	testCanEncode(t, e, encoding.CanEncodeFloat)
}

func testCanEncodeDouble(t testing.TB, e encoding.Encoding) {
	testCanEncode(t, e, encoding.CanEncodeDouble)
}

func testCanEncodeByteArray(t testing.TB, e encoding.Encoding) {
	testCanEncode(t, e, encoding.CanEncodeByteArray)
}

func testCanEncodeFixedLenByteArray(t testing.TB, e encoding.Encoding) {
	testCanEncode(t, e, encoding.CanEncodeFixedLenByteArray)
}

func testCanEncode(t testing.TB, e encoding.Encoding, test func(encoding.Encoding) bool) {
	if !test(e) {
		t.Skip("encoding not supported")
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func assertBytesEqual(t *testing.T, want, got []byte) {
	t.Helper()
	if !bytes.Equal(want, got) {
		t.Fatalf("values mismatch:\nwant = %q\ngot  = %q", want, got)
	}
}

const (
	benchmarkNumValues = 10e3
)

func newRand() *rand.Rand {
	return rand.New(rand.NewSource(1))
}

func BenchmarkEncode(b *testing.B) {
	for _, encoding := range encodings {
		b.Run(encoding.String(), func(b *testing.B) { benchmarkEncode(b, encoding) })
	}
}

func benchmarkEncode(b *testing.B, e encoding.Encoding) {
	for _, test := range [...]struct {
		scenario string
		function func(*testing.B, encoding.Encoding)
	}{
		{
			scenario: "boolean",
			function: benchmarkEncodeBoolean,
		},
		{
			scenario: "levels",
			function: benchmarkEncodeLevels,
		},
		{
			scenario: "int32",
			function: benchmarkEncodeInt32,
		},
		{
			scenario: "int64",
			function: benchmarkEncodeInt64,
		},
		{
			scenario: "float",
			function: benchmarkEncodeFloat,
		},
		{
			scenario: "double",
			function: benchmarkEncodeDouble,
		},
		{
			scenario: "byte array",
			function: benchmarkEncodeByteArray,
		},
		{
			scenario: "fixed length byte array",
			function: benchmarkEncodeFixedLenByteArray,
		},
	} {
		b.Run(test.scenario, func(b *testing.B) { test.function(b, e) })
	}
}

func benchmarkEncodeBoolean(b *testing.B, e encoding.Encoding) {
	testCanEncodeBoolean(b, e)
	buffer := make([]byte, 0)
	values := generateBooleanValues(benchmarkNumValues, newRand())
	setBitWidth(e, 1)

	reportThroughput(b, benchmarkNumValues, len(values), func() {
		benchmarkZeroAllocsPerRun(b, func() {
			buffer, _ = e.EncodeBoolean(buffer, values)
		})
	})
}

func benchmarkEncodeLevels(b *testing.B, e encoding.Encoding) {
	testCanEncodeLevels(b, e)
	buffer := make([]byte, 0)
	values := generateLevelValues(benchmarkNumValues, newRand())
	setBitWidth(e, maxLenInt8(unsafecast.BytesToInt8(values)))

	reportThroughput(b, benchmarkNumValues, len(values), func() {
		benchmarkZeroAllocsPerRun(b, func() {
			buffer, _ = e.EncodeLevels(buffer, values)
		})
	})
}

func benchmarkEncodeInt32(b *testing.B, e encoding.Encoding) {
	testCanEncodeInt32(b, e)
	buffer := make([]byte, 0)
	values := generateInt32Values(benchmarkNumValues, newRand())
	setBitWidth(e, maxLenInt32(unsafecast.BytesToInt32(values)))

	reportThroughput(b, benchmarkNumValues, len(values), func() {
		benchmarkZeroAllocsPerRun(b, func() {
			buffer, _ = e.EncodeInt32(buffer, values)
		})
	})
}

func benchmarkEncodeInt64(b *testing.B, e encoding.Encoding) {
	testCanEncodeInt64(b, e)
	buffer := make([]byte, 0)
	values := generateInt64Values(benchmarkNumValues, newRand())
	setBitWidth(e, maxLenInt64(unsafecast.BytesToInt64(values)))

	reportThroughput(b, benchmarkNumValues, len(values), func() {
		benchmarkZeroAllocsPerRun(b, func() {
			buffer, _ = e.EncodeInt64(buffer, values)
		})
	})
}

func benchmarkEncodeFloat(b *testing.B, e encoding.Encoding) {
	testCanEncodeFloat(b, e)
	buffer := make([]byte, 0)
	values := generateFloatValues(benchmarkNumValues, newRand())

	reportThroughput(b, benchmarkNumValues, len(values), func() {
		benchmarkZeroAllocsPerRun(b, func() {
			buffer, _ = e.EncodeFloat(buffer, values)
		})
	})
}

func benchmarkEncodeDouble(b *testing.B, e encoding.Encoding) {
	testCanEncodeDouble(b, e)
	buffer := make([]byte, 0)
	values := generateDoubleValues(benchmarkNumValues, newRand())

	reportThroughput(b, benchmarkNumValues, len(values), func() {
		benchmarkZeroAllocsPerRun(b, func() {
			buffer, _ = e.EncodeDouble(buffer, values)
		})
	})
}

func benchmarkEncodeByteArray(b *testing.B, e encoding.Encoding) {
	testCanEncodeByteArray(b, e)
	buffer := make([]byte, 0)
	values := generateByteArrayValues(benchmarkNumValues, newRand())

	reportThroughput(b, benchmarkNumValues, len(values), func() {
		benchmarkZeroAllocsPerRun(b, func() {
			buffer, _ = e.EncodeByteArray(buffer, values)
		})
	})
}

func benchmarkEncodeFixedLenByteArray(b *testing.B, e encoding.Encoding) {
	testCanEncodeFixedLenByteArray(b, e)
	const size = 16
	buffer := make([]byte, 0)
	values := generateFixedLenByteArrayValues(benchmarkNumValues, newRand(), size)

	reportThroughput(b, benchmarkNumValues, len(values), func() {
		benchmarkZeroAllocsPerRun(b, func() {
			buffer, _ = e.EncodeFixedLenByteArray(buffer, values, size)
		})
	})
}

func BenchmarkDecode(b *testing.B) {
	for _, encoding := range encodings {
		b.Run(encoding.String(), func(b *testing.B) { benchmarkDecode(b, encoding) })
	}
}

func benchmarkDecode(b *testing.B, e encoding.Encoding) {
	for _, test := range [...]struct {
		scenario string
		function func(*testing.B, encoding.Encoding)
	}{
		{
			scenario: "boolean",
			function: benchmarkDecodeBoolean,
		},
		{
			scenario: "levels",
			function: benchmarkDecodeLevels,
		},
		{
			scenario: "int32",
			function: benchmarkDecodeInt32,
		},
		{
			scenario: "int64",
			function: benchmarkDecodeInt64,
		},
		{
			scenario: "float",
			function: benchmarkDecodeFloat,
		},
		{
			scenario: "double",
			function: benchmarkDecodeDouble,
		},
		{
			scenario: "byte array",
			function: benchmarkDecodeByteArray,
		},
		{
			scenario: "fixed length byte array",
			function: benchmarkDecodeFixedLenByteArray,
		},
	} {
		b.Run(test.scenario, func(b *testing.B) { test.function(b, e) })
	}
}

func benchmarkDecodeBoolean(b *testing.B, e encoding.Encoding) {
	testCanEncodeBoolean(b, e)
	values := generateBooleanValues(benchmarkNumValues, newRand())
	output := make([]byte, 0)
	setBitWidth(e, 1)
	buffer, _ := e.EncodeBoolean(nil, values)

	reportThroughput(b, benchmarkNumValues, len(values), func() {
		benchmarkZeroAllocsPerRun(b, func() {
			output, _ = e.DecodeBoolean(output, buffer)
		})
	})
}

func benchmarkDecodeLevels(b *testing.B, e encoding.Encoding) {
	testCanEncodeLevels(b, e)
	values := generateLevelValues(benchmarkNumValues, newRand())
	output := make([]byte, 0)
	setBitWidth(e, maxLenInt8(unsafecast.BytesToInt8(values)))
	buffer, _ := e.EncodeLevels(nil, values)

	reportThroughput(b, benchmarkNumValues, len(values), func() {
		benchmarkZeroAllocsPerRun(b, func() {
			output, _ = e.DecodeLevels(output, buffer)
		})
	})
}

func benchmarkDecodeInt32(b *testing.B, e encoding.Encoding) {
	testCanEncodeInt32(b, e)
	values := generateInt32Values(benchmarkNumValues, newRand())
	output := make([]byte, 0)
	setBitWidth(e, maxLenInt32(unsafecast.BytesToInt32(values)))
	buffer, _ := e.EncodeInt32(nil, values)

	reportThroughput(b, benchmarkNumValues, len(values), func() {
		benchmarkZeroAllocsPerRun(b, func() {
			output, _ = e.DecodeInt32(output, buffer)
		})
	})
}

func benchmarkDecodeInt64(b *testing.B, e encoding.Encoding) {
	testCanEncodeInt64(b, e)
	values := generateInt64Values(benchmarkNumValues, newRand())
	output := make([]byte, 0)
	setBitWidth(e, maxLenInt64(unsafecast.BytesToInt64(values)))
	buffer, _ := e.EncodeInt64(nil, values)

	reportThroughput(b, benchmarkNumValues, len(values), func() {
		benchmarkZeroAllocsPerRun(b, func() {
			output, _ = e.DecodeInt64(output, buffer)
		})
	})
}

func benchmarkDecodeFloat(b *testing.B, e encoding.Encoding) {
	testCanEncodeFloat(b, e)
	values := generateFloatValues(benchmarkNumValues, newRand())
	output := make([]byte, 0)
	buffer, _ := e.EncodeFloat(nil, values)

	reportThroughput(b, benchmarkNumValues, len(values), func() {
		benchmarkZeroAllocsPerRun(b, func() {
			output, _ = e.DecodeFloat(output, buffer)
		})
	})
}

func benchmarkDecodeDouble(b *testing.B, e encoding.Encoding) {
	testCanEncodeDouble(b, e)
	values := generateDoubleValues(benchmarkNumValues, newRand())
	output := make([]byte, 0)
	buffer, _ := e.EncodeDouble(nil, values)

	reportThroughput(b, benchmarkNumValues, len(values), func() {
		benchmarkZeroAllocsPerRun(b, func() {
			output, _ = e.DecodeDouble(output, buffer)
		})
	})
}

func benchmarkDecodeByteArray(b *testing.B, e encoding.Encoding) {
	testCanEncodeByteArray(b, e)
	values := generateByteArrayValues(benchmarkNumValues, newRand())
	output := make([]byte, 0)
	buffer, _ := e.EncodeByteArray(nil, values)

	reportThroughput(b, benchmarkNumValues, len(values), func() {
		benchmarkZeroAllocsPerRun(b, func() {
			output, _ = e.DecodeByteArray(output, buffer)
		})
	})
}

func benchmarkDecodeFixedLenByteArray(b *testing.B, e encoding.Encoding) {
	testCanEncodeFixedLenByteArray(b, e)
	const size = 16
	values := generateFixedLenByteArrayValues(benchmarkNumValues, newRand(), size)
	output := make([]byte, 0)
	buffer, _ := e.EncodeFixedLenByteArray(nil, values, size)

	reportThroughput(b, benchmarkNumValues, len(values), func() {
		benchmarkZeroAllocsPerRun(b, func() {
			output, _ = e.DecodeFixedLenByteArray(output, buffer, size)
		})
	})
}

func benchmarkZeroAllocsPerRun(b *testing.B, f func()) {
	if allocs := testing.AllocsPerRun(b.N, f); allocs != 0 && !testing.Short() {
		b.Errorf("too many memory allocations: %g", allocs)
	}
}

func reportThroughput(b *testing.B, numValues, numBytes int, do func()) {
	start := time.Now()
	do()
	seconds := time.Since(start).Seconds()
	b.SetBytes(int64(numBytes))
	b.ReportMetric(float64(b.N*numValues)/seconds, "value/s")
}

func generateBooleanValues(n int, r *rand.Rand) []byte {
	values := make([]byte, n/8+1)
	io.ReadFull(r, values)
	return values
}

func generateLevelValues(n int, r *rand.Rand) []byte {
	values := make([]byte, n)
	for i := range values {
		values[i] = byte(r.Intn(6))
	}
	return values
}

func generateInt32Values(n int, r *rand.Rand) []byte {
	values := make([]byte, 4*n)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint32(values[i*4:], uint32(r.Int31n(100)))
	}
	return values
}

func generateInt64Values(n int, r *rand.Rand) []byte {
	values := make([]byte, 8*n)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint64(values[i*8:], uint64(r.Int63n(100)))
	}
	return values
}

func generateFloatValues(n int, r *rand.Rand) []byte {
	values := make([]byte, 4*n)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint32(values[i*4:], math.Float32bits(r.Float32()))
	}
	return values
}

func generateDoubleValues(n int, r *rand.Rand) []byte {
	values := make([]byte, 8*n)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint64(values[i*8:], math.Float64bits(r.Float64()))
	}
	return values
}

func generateByteArrayValues(n int, r *rand.Rand) []byte {
	const maxLen = 21
	values := make([]byte, plain.ByteArrayLengthSize*n+n*maxLen)
	length := 0

	for i := 0; i < n; i++ {
		k := r.Intn(maxLen) + 1
		plain.PutByteArrayLength(values[length:], k)
		length += plain.ByteArrayLengthSize
		io.ReadFull(r, values[length:length+k])
		length += k
	}

	return values[:length]
}

func generateFixedLenByteArrayValues(n int, r *rand.Rand, size int) []byte {
	values := make([]byte, n*size)
	io.ReadFull(r, values)
	return values
}

func maxLenInt8(data []int8) int {
	max := 0
	for _, v := range data {
		if n := bits.Len8(uint8(v)); n > max {
			max = n
		}
	}
	return max
}

func maxLenInt32(data []int32) int {
	max := 0
	for _, v := range data {
		if n := bits.Len32(uint32(v)); n > max {
			max = n
		}
	}
	return max
}

func maxLenInt64(data []int64) int {
	max := 0
	for _, v := range data {
		if n := bits.Len64(uint64(v)); n > max {
			max = n
		}
	}
	return max
}
