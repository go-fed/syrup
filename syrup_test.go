package syrup

import (
	"bytes"
	"fmt"
	"math/big"
	"reflect"
	"testing"
)

// TODO: Unterminated list error test

type Struct1 struct {
	I    int
	f    float32
	Do   float64
	unum uint64
	Str  string
}

type Struct2 struct {
	S1     *Struct1
	sUnexp *Struct1
}

type Struct3 struct {
	Struct1
}

type Struct4 struct {
	*Struct1
}

type test struct {
	name string
	// If goValueExpect is set, decode will test against it (good for
	// unexported fields).
	goValue       interface{}
	goValueExpect interface{}
	decode        interface{}
	// Either set encoding or encodings. Not both. Encodings is there due to
	// golang's map non-determinism.
	encoding  []byte
	encodings [][]byte
}

// Addressable vars for tests
var astring string
var abyte []byte
var aint8 int8
var aint16 int16
var aint32 int32
var aint64 int64
var abigint *big.Int
var aint int
var auint8 uint8
var auint16 uint16
var auint32 uint32
var auint64 uint64
var auint uint
var abool bool
var afloat32 float32
var afloat64 float64
var asymbol Symbol
var arecord Record
var astringarr []string
var aset Set
var aMapStringInt map[string]int
var astruct1 Struct1
var astruct2 Struct2
var astruct3 Struct3
var astruct4 Struct4
var ainterface interface{}

func resetAddressables() {
	abyte = nil
	astringarr = nil
	aMapStringInt = nil
	ainterface = nil
}

var tests = []test{
	{
		name:     "String",
		goValue:  "Hello, World!",
		encoding: []byte("13\"Hello, World!"),
		decode:   &astring,
	},
	{
		name:     "Int8",
		goValue:  int8(5),
		encoding: []byte("i5e"),
		decode:   &aint8,
	},
	{
		name:     "Int16",
		goValue:  int16(128),
		encoding: []byte("i128e"),
		decode:   &aint16,
	},
	{
		name:     "Int32",
		goValue:  int32(32768),
		encoding: []byte("i32768e"),
		decode:   &aint32,
	},
	{
		name:     "Int64",
		goValue:  int64(2147483648),
		encoding: []byte("i2147483648e"),
		decode:   &aint64,
	},
	{
		name:     "BigInt",
		goValue:  big.NewInt(0).Mul(big.NewInt(9223372036854775807), big.NewInt(10)),
		encoding: []byte("i92233720368547758070e"),
		decode:   &abigint,
	},
	{
		name:     "Negative Int",
		goValue:  int(-919),
		encoding: []byte("i-919e"),
		decode:   &aint,
	},
	{
		name:     "Uint8",
		goValue:  uint8(5),
		encoding: []byte("i5e"),
		decode:   &auint8,
	},
	{
		name:     "Uint16",
		goValue:  uint16(128),
		encoding: []byte("i128e"),
		decode:   &auint16,
	},
	{
		name:     "Unt32",
		goValue:  uint32(32768),
		encoding: []byte("i32768e"),
		decode:   &auint32,
	},
	{
		name:     "Uint64",
		goValue:  uint64(2147483648),
		encoding: []byte("i2147483648e"),
		decode:   &auint64,
	},
	{
		name:     "Uint",
		goValue:  uint(919),
		encoding: []byte("i919e"),
		decode:   &auint,
	},
	{
		name:     "True",
		goValue:  true,
		encoding: []byte("t"),
		decode:   &abool,
	},
	{
		name:     "False",
		goValue:  false,
		encoding: []byte("f"),
		decode:   &abool,
	},
	{
		name:     "Float32",
		goValue:  float32(3.14159),
		encoding: []byte{'F', 64, 73, 15, 208},
		decode:   &afloat32,
	},
	{
		name:     "Float64",
		goValue:  float64(3.14159),
		encoding: []byte{'D', 64, 9, 33, 249, 240, 27, 134, 110},
		decode:   &afloat64,
	},
	{
		name:     "Symbol",
		goValue:  Symbol("PtrToIt"),
		encoding: []byte("7'PtrToIt"),
		decode:   &asymbol,
	},
	{
		name: "Record",
		goValue: Record{
			Label: "Napalm",
			Values: []interface{}{
				int64(-5),
				Symbol("Yep"),
				"Hello",
			},
		},
		encoding: []byte("<6\"Napalmi-5e3'Yep5\"Hello>"),
		decode:   &arecord,
	},
	{
		name:     "[]byte",
		goValue:  []byte{1, 1, 2, 3, 5, 8, 13, 8, 5, 3, 2, 1, 1},
		encoding: []byte{'1', '3', ':', 1, 1, 2, 3, 5, 8, 13, 8, 5, 3, 2, 1, 1},
		decode:   &abyte,
	},
	{
		name: "[]byte in list into interface",
		goValue: []interface{}{
			"str",
			5,
			[]byte{49, 50, 51},
			[]byte{54, 55, 56, 57},
		},
		goValueExpect: []interface{}{
			"str",
			int64(5),
			[]byte{49, 50, 51},
			[]byte{54, 55, 56, 57},
		},
		encoding: []byte("[3\"stri5e3:1234:6789]"),
		decode: &ainterface,
	},
	{
		name:     "[]string",
		goValue:  []string{"Hello", "World!"},
		encoding: []byte("[5\"Hello6\"World!]"),
		decode:   &astringarr,
	},
	{
		name:     "Set",
		goValue:  Set([]interface{}{"Hello", int64(42)}),
		encoding: []byte("#5\"Helloi42e$"),
		decode:   &aset,
	},
	{
		name:    "map[string]int",
		goValue: map[string]int{"in": 2, "out of here": -99},
		encodings: [][]byte{
			[]byte("{2\"ini2e11\"out of herei-99e}"),
			[]byte("{11\"out of herei-99e2\"ini2e}"),
		},
		decode: &aMapStringInt,
	},
	{
		name: "Struct1",
		goValue: Struct1{
			I:    -5,
			f:    4.321,
			Do:   3.14159,
			unum: 256,
			Str:  "Life's a bitch, then you rejuvenate and do it all over again.",
		},
		goValueExpect: Struct1{
			I:   -5,
			Do:  3.14159,
			Str: "Life's a bitch, then you rejuvenate and do it all over again.",
		},
		encoding: append(
			append([]byte("{1\"Ii-5e2\"Do"),
				[]byte{'D', 64, 9, 33, 249, 240, 27, 134, 110}...),
			[]byte("3\"Str61\"Life's a bitch, then you rejuvenate and do it all over again.}")...),
		decode: &astruct1,
	},
	{
		name: "Struct2",
		goValue: Struct2{
			S1: &Struct1{
				I:    -5,
				f:    4.321,
				Do:   3.14159,
				unum: 256,
				Str:  "Life's a bitch, then you rejuvenate and do it all over again.",
			},
			sUnexp: &Struct1{
				I:    1,
				f:    1.28,
				Do:   1.2825,
				unum: 128,
				Str:  "How you humans survive so much experience is something I shall never understand.",
			},
		},
		goValueExpect: Struct2{
			S1: &Struct1{
				I:   -5,
				Do:  3.14159,
				Str: "Life's a bitch, then you rejuvenate and do it all over again.",
			},
		},
		encoding: append(
			append([]byte("{2\"S1{1\"Ii-5e2\"Do"),
				[]byte{'D', 64, 9, 33, 249, 240, 27, 134, 110}...),
			[]byte("3\"Str61\"Life's a bitch, then you rejuvenate and do it all over again.}}")...),
		decode: &astruct2,
	},
	{
		name: "Struct3",
		goValue: Struct3{
			Struct1{
				I:    -5,
				f:    4.321,
				Do:   3.14159,
				unum: 256,
				Str:  "Life's a bitch, then you rejuvenate and do it all over again.",
			},
		},
		goValueExpect: Struct3{
			Struct1{
				I:   -5,
				Do:  3.14159,
				Str: "Life's a bitch, then you rejuvenate and do it all over again.",
			},
		},
		encoding: append(
			append([]byte("{7\"Struct1{1\"Ii-5e2\"Do"),
				[]byte{'D', 64, 9, 33, 249, 240, 27, 134, 110}...),
			[]byte("3\"Str61\"Life's a bitch, then you rejuvenate and do it all over again.}}")...),
		decode: &astruct3,
	},
	{
		name: "Struct4",
		goValue: Struct4{
			&Struct1{
				I:    -5,
				f:    4.321,
				Do:   3.14159,
				unum: 256,
				Str:  "Life's a bitch, then you rejuvenate and do it all over again.",
			},
		},
		goValueExpect: Struct4{
			&Struct1{
				I:   -5,
				Do:  3.14159,
				Str: "Life's a bitch, then you rejuvenate and do it all over again.",
			},
		},
		encoding: append(
			append([]byte("{7\"Struct1{1\"Ii-5e2\"Do"),
				[]byte{'D', 64, 9, 33, 249, 240, 27, 134, 110}...),
			[]byte("3\"Str61\"Life's a bitch, then you rejuvenate and do it all over again.}}")...),
		decode: &astruct4,
	},
}

func TestEncode(t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var buf bytes.Buffer
			enc := NewEncoder(NewPrototypeEncoding(), &buf)
			err := enc.Encode(test.goValue)
			if err != nil {
				t.Errorf("got error %v", err)
			} else if test.encodings != nil {
				ok := false
				for _, encoding := range test.encodings {
					ok = ok || bytes.Equal(buf.Bytes(), encoding)
				}
				if !ok {
					t.Errorf("got %v, which matched no encodings", buf.Bytes())
					t.Errorf("len got %d", len(buf.Bytes()))
				}
			} else if !bytes.Equal(buf.Bytes(), test.encoding) {
				t.Errorf("got %v, encoding %v", buf.Bytes(), test.encoding)
				t.Errorf("len got %d, len encoding %d", len(buf.Bytes()), len(test.encoding))
			}
		})
		ptrName := "Pointer To " + test.name
		t.Run(ptrName, func(t *testing.T) {
			var buf bytes.Buffer
			enc := NewEncoder(NewPrototypeEncoding(), &buf)
			err := enc.Encode(&test.goValue)
			if err != nil {
				t.Errorf("got error %v", err)
			} else if test.encodings != nil {
				ok := false
				for _, encoding := range test.encodings {
					ok = ok || bytes.Equal(buf.Bytes(), encoding)
				}
				if !ok {
					t.Errorf("got %v, which matched no encodings", buf.Bytes())
					t.Errorf("len got %d", len(buf.Bytes()))
				}
			} else if !bytes.Equal(buf.Bytes(), test.encoding) {
				t.Errorf("got %v, encoding %v", buf.Bytes(), test.encoding)
				t.Errorf("len got %d, len encoding %d", len(buf.Bytes()), len(test.encoding))
			}
		})
	}
}

func TestDecode(t *testing.T) {
	for _, test := range tests {
		if len(test.encoding) > 0 {
			t.Run(test.name, func(t *testing.T) {
				resetAddressables()
				buf := bytes.NewBuffer([]byte(test.encoding))
				dec := NewDecoder(NewPrototypeEncoding(), buf)
				err := dec.Decode(test.decode)
				if err != nil {
					t.Errorf("got error %v", err)
					return
				}
				pme := reflect.ValueOf(test.decode)
				expect := test.goValue
				if test.goValueExpect != nil {
					expect = test.goValueExpect
				}
				if !reflect.DeepEqual(pme.Elem().Interface(), expect) {
					t.Errorf("got %v, want %v", pme.Elem().Interface(), test.goValue)
				}
			})
		} else {
			for i, encoding := range test.encodings {
				t.Run(fmt.Sprintf("%s_%d", test.name, i), func(t *testing.T) {
					resetAddressables()
					buf := bytes.NewBuffer([]byte(encoding))
					dec := NewDecoder(NewPrototypeEncoding(), buf)
					err := dec.Decode(test.decode)
					if err != nil {
						t.Errorf("got error %v", err)
						return
					}
					pme := reflect.ValueOf(test.decode)
					expect := test.goValue
					if test.goValueExpect != nil {
						expect = test.goValueExpect
					}
					if !reflect.DeepEqual(pme.Elem().Interface(), expect) {
						t.Errorf("got %v, want %v", pme.Elem().Interface(), test.goValue)
					}
				})
			}
		}
	}
}

func TestDecodeInterface(t *testing.T) {
	buf := bytes.NewBuffer([]byte("[5\"Helloi42e]"))
	dec := NewDecoder(NewPrototypeEncoding(), buf)
	var v interface{}
	err := dec.Decode(&v)
	if err != nil {
		t.Errorf("got error %v", err)
		return
	}
	expected := []interface{}{"Hello", int64(42)}
	if varr, ok := v.([]interface{}); !ok {
		t.Errorf("v is not a slice")
	} else if len(varr) != len(expected) {
		t.Errorf("len: got %#v, want %#v", v, expected)
	} else if varr[0] != expected[0] {
		t.Errorf("0: got %#v, want %#v", v, expected)
		t.Errorf("0: got %T, want %T", varr[0], expected[0])
	} else if varr[1] != expected[1] {
		t.Errorf("1: got %#v, want %#v", v, expected)
		t.Errorf("1: got %T, want %T", varr[1], expected[1])
	}
}

func TestDecodeArraySilentDrop(t *testing.T) {
	buf := bytes.NewBuffer([]byte("[5\"Hello7\"PtrToIt6\"World!]"))
	dec := NewDecoder(NewPrototypeEncoding(), buf)
	var s [2]string
	err := dec.Decode(&s)
	if err != nil {
		t.Errorf("got error %v", err)
		return
	}
	expected := [2]string{"Hello", "PtrToIt"}
	if !reflect.DeepEqual(s, expected) {
		t.Errorf("got %v, want %v", s, expected)
	}
}

func TestDecodeArrayExtrasZero(t *testing.T) {
	buf := bytes.NewBuffer([]byte("[5\"Hello7\"PtrToIt6\"World!]"))
	dec := NewDecoder(NewPrototypeEncoding(), buf)
	var s [5]string
	err := dec.Decode(&s)
	if err != nil {
		t.Errorf("got error %v", err)
		return
	}
	expected := [5]string{"Hello", "PtrToIt", "World!", "", ""}
	if !reflect.DeepEqual(s, expected) {
		t.Errorf("got %v, want %v", s, expected)
	}
}

func TestDecodeArrayNilInterface(t *testing.T) {
	buf := bytes.NewBuffer([]byte("[5\"Hello7\"PtrToIt6\"World!]"))
	dec := NewDecoder(NewPrototypeEncoding(), buf)
	var v interface{}
	err := dec.Decode(&v)
	if err != nil {
		t.Errorf("got error %v", err)
		return
	}
	expected := []interface{}{"Hello", "PtrToIt", "World!"}
	if !reflect.DeepEqual(v, expected) {
		t.Errorf("got %v, want %v", v, expected)
	}
}
