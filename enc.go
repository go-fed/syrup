package syrup

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"unicode"
)

type Encoding struct {
	fmtString         func(s string) []byte
	fmtBigInt         func(i *big.Int) []byte
	fmtInt            func(i int64) []byte
	fmtUint           func(i uint64) []byte
	fmtBool           func(b bool) []byte
	fmtFloat64        func(f float64) []byte
	fmtFloat32        func(f float32) []byte
	fmtBytes          func(b []byte) []byte
	fmtSymbol         func(s string) []byte
	listOpen          func() []byte
	listClose         func() []byte
	dictOpen          func() []byte
	dictClose         func() []byte
	setOpen           func() []byte
	setClose          func() []byte
	recordOpen        func() []byte
	recordClose       func() []byte
	mustFindToken     func(b byte) (scanState, op, bool, error)
	scanTokenLen      func(b byte) (scanState, op, bool, error)
	scanFirstIntToken func(b byte) (scanState, op, bool, error)
	scanIntToken      func(b byte) (scanState, op, bool, error)
	scanFloat64Token  func(b byte) (scanState, op, bool, error)
	scanFloat32Token  func(b byte) (scanState, op, bool, error)
	parseLen          func(s string, next scanState) (op, uint64, error)
	boolVal           func(b []byte) (bool, error)
	symbolVal         func(b []byte) (Symbol, error)
	stringVal         func(b []byte) (string, error)
	int64Val          func(b []byte) (int64, *big.Int, error)
	float32Val        func(b []byte) (float32, error)
	float64Val        func(b []byte) (float64, error)
	nlen              uint8 // For counting lengths of floating point encodings.
}

// NewPrototypeEncoding returns the prototypical syrup encoding proposed.
func NewPrototypeEncoding() *Encoding {
	e := &Encoding{
		fmtString:         syrupProtoString,
		fmtBigInt:         syrupProtoBigInt,
		fmtInt:            syrupProtoInt,
		fmtUint:           syrupProtoUint,
		fmtBool:           syrupProtoBool,
		fmtFloat64:        syrupProtoFloat64,
		fmtFloat32:        syrupProtoFloat32,
		fmtBytes:          syrupProtoBytes,
		fmtSymbol:         syrupProtoSymbol,
		listOpen:          syrupProtoListOpen,
		listClose:         syrupProtoListClose,
		dictOpen:          syrupProtoDictOpen,
		dictClose:         syrupProtoDictClose,
		setOpen:           syrupProtoSetOpen,
		setClose:          syrupProtoSetClose,
		recordOpen:        syrupProtoRecordOpen,
		recordClose:       syrupProtoRecordClose,
		scanTokenLen:      syrupProtoScanTokenLen,
		scanFirstIntToken: syrupProtoScanFirstIntToken,
		scanIntToken:      syrupProtoScanIntToken,
		parseLen:          syrupProtoParsedLen,
		boolVal:           syrupProtoBoolVal,
		symbolVal:         syrupProtoSymbolVal,
		stringVal:         syrupProtoStringVal,
		int64Val:          syrupProtoInt64Val,
		float32Val:        syrupProtoFloat32Val,
		float64Val:        syrupProtoFloat64Val,
		nlen:              0,
	}
	e.mustFindToken = func(b byte) (scanState, op, bool, error) {
		s, o, in, err := syrupProtoMustFindToken(b)
		if s == scanFloat32 {
			e.nlen = 4
		} else if s == scanFloat64 {
			e.nlen = 8
		}
		return s, o, in, err
	}
	e.scanFloat64Token = func(b byte) (scanState, op, bool, error) {
		if e.nlen == 0 {
			return scanFindToken, noop, false, errors.New("missing float64 syrup delimiter or too many calls to parse float64")
		}
		e.nlen--
		return syrupProtoScanFloat64Token(e.nlen)
	}
	e.scanFloat32Token = func(b byte) (scanState, op, bool, error) {
		if e.nlen == 0 {
			return scanFindToken, noop, false, errors.New("missing float32 syrup delimiter or too many calls to parse float32")
		}
		e.nlen--
		return syrupProtoScanFloat32Token(e.nlen)
	}
	return e
}

func syrupProtoString(s string) []byte {
	b := append([]byte(strconv.FormatInt(int64(len(s)), 10)), '"')
	b = append(b, []byte(s)...)
	return b
}

func syrupProtoBigInt(i *big.Int) []byte {
	b := append([]byte{'i'},
		[]byte(i.Text(10))...)
	b = append(b, 'e')
	return b
}

func syrupProtoInt(i int64) []byte {
	b := append([]byte{'i'},
		[]byte(strconv.FormatInt(i, 10))...)
	b = append(b, 'e')
	return b
}

func syrupProtoUint(i uint64) []byte {
	b := append([]byte{'i'},
		[]byte(strconv.FormatUint(i, 10))...)
	b = append(b, 'e')
	return b
}

func syrupProtoBool(b bool) []byte {
	if b {
		return []byte{'t'}
	} else {
		return []byte{'f'}
	}
}

func syrupProtoFloat64(f float64) []byte {
	b := make([]byte, 9)
	b[0] = 'D'
	binary.BigEndian.PutUint64(b[1:], math.Float64bits(f))
	return b
}

func syrupProtoFloat32(f float32) []byte {
	b := make([]byte, 5)
	b[0] = 'F'
	binary.BigEndian.PutUint32(b[1:], math.Float32bits(f))
	return b
}

func syrupProtoBytes(s []byte) []byte {
	b := append([]byte(strconv.FormatInt(int64(len(s)), 10)), ':')
	b = append(b, s...)
	return b
}

func syrupProtoListOpen() []byte {
	return []byte{'['}
}

func syrupProtoListClose() []byte {
	return []byte{']'}
}

func syrupProtoDictOpen() []byte {
	return []byte{'{'}
}

func syrupProtoDictClose() []byte {
	return []byte{'}'}
}

func syrupProtoSymbol(s string) []byte {
	b := append([]byte(strconv.FormatInt(int64(len(s)), 10)), '\'')
	b = append(b, []byte(s)...)
	return b
}

func syrupProtoSetOpen() []byte {
	return []byte{'#'}
}

func syrupProtoSetClose() []byte {
	return []byte{'$'}
}

func syrupProtoRecordOpen() []byte {
	return []byte{'<'}
}

func syrupProtoRecordClose() []byte {
	return []byte{'>'}
}

// Determines the next scan state, whether to use the passed-in byte as part of
// further processing, and any errors.
func syrupProtoMustFindToken(b byte) (scanState, op, bool, error) {
	if unicode.IsSpace(rune(b)) {
		return scanFindToken, noop, false, nil
	}
	switch b {
	case '0':
		fallthrough
	case '1':
		fallthrough
	case '2':
		fallthrough
	case '3':
		fallthrough
	case '4':
		fallthrough
	case '5':
		fallthrough
	case '6':
		fallthrough
	case '7':
		fallthrough
	case '8':
		fallthrough
	case '9':
		return scanTokenLen, noop, true, nil
	case 'i':
		return scanFirstInt, noop, false, nil
	case 't':
		fallthrough
	case 'f':
		return scanFindToken, valBoolop, true, nil
	case 'F':
		return scanFloat32, noop, false, nil
	case 'D':
		return scanFloat64, noop, false, nil
	case '[':
		return scanFindToken, openListOp, false, nil
	case '{':
		return scanFindToken, openDictOp, false, nil
	case '#':
		return scanFindToken, openSetOp, false, nil
	case '<':
		return scanFindToken, openRecordOp, false, nil
	case ']':
		return scanFindToken, closeListOp, false, nil
	case '}':
		return scanFindToken, closeDictOp, false, nil
	case '$':
		return scanFindToken, closeSetOp, false, nil
	case '>':
		return scanFindToken, closeRecordOp, false, nil
	default:
		return scanFindToken, noop, false, fmt.Errorf("could not determine token for byte: %v", b)
	}
}

func syrupProtoScanTokenLen(b byte) (scanState, op, bool, error) {
	switch b {
	case '0':
		fallthrough
	case '1':
		fallthrough
	case '2':
		fallthrough
	case '3':
		fallthrough
	case '4':
		fallthrough
	case '5':
		fallthrough
	case '6':
		fallthrough
	case '7':
		fallthrough
	case '8':
		fallthrough
	case '9':
		return scanTokenLen, noop, true, nil
	case '\'':
		return scanSymbol, noop, false, nil
	case '"':
		return scanString, noop, false, nil
	case ':':
		return scanByteArr, noop, false, nil
	default:
		return scanFindToken, noop, false, fmt.Errorf("malformed input during len scanning: %v", b)
	}
}

func syrupProtoScanFirstIntToken(b byte) (scanState, op, bool, error) {
	switch b {
	case '-':
		fallthrough
	case '0':
		fallthrough
	case '1':
		fallthrough
	case '2':
		fallthrough
	case '3':
		fallthrough
	case '4':
		fallthrough
	case '5':
		fallthrough
	case '6':
		fallthrough
	case '7':
		fallthrough
	case '8':
		fallthrough
	case '9':
		return scanInt, noop, true, nil
	case 'e':
		return scanFindToken, valIntOp, false, nil
	default:
		return scanFindToken, noop, false, fmt.Errorf("malformed input during int scanning: %v", b)
	}
}

func syrupProtoScanIntToken(b byte) (scanState, op, bool, error) {
	switch b {
	case '0':
		fallthrough
	case '1':
		fallthrough
	case '2':
		fallthrough
	case '3':
		fallthrough
	case '4':
		fallthrough
	case '5':
		fallthrough
	case '6':
		fallthrough
	case '7':
		fallthrough
	case '8':
		fallthrough
	case '9':
		return scanInt, noop, true, nil
	case 'e':
		return scanFindToken, valIntOp, false, nil
	default:
		return scanFindToken, noop, false, fmt.Errorf("malformed input during int scanning: %v", b)
	}
}

func syrupProtoScanFloat32Token(n uint8) (scanState, op, bool, error) {
	if n == 0 {
		return scanFindToken, valFloat32Op, true, nil
	} else {
		return scanFloat32, noop, true, nil
	}
}

func syrupProtoScanFloat64Token(n uint8) (scanState, op, bool, error) {
	if n == 0 {
		return scanFindToken, valFloat64Op, true, nil
	} else {
		return scanFloat64, noop, true, nil
	}
}

func syrupProtoParsedLen(s string, next scanState) (do op, l uint64, err error) {
	l, err = strconv.ParseUint(s, 10, 64)
	if err != nil {
		return
	}
	if l == 0 {
		switch next {
		case scanSymbol:
			do = valSymbolOp
		case scanString:
			do = valStringOp
		case scanByteArr:
			do = valByteArrOp
		default:
			err = fmt.Errorf("syrup len parsing bad state: %v", next)
		}
	}
	return
}

func syrupProtoBoolVal(b []byte) (bool, error) {
	if len(b) != 1 {
		return false, fmt.Errorf("syrup bool val len %d", len(b))
	}
	if b[0] == 't' {
		return true, nil
	} else if b[0] == 'f' {
		return false, nil
	}
	return false, fmt.Errorf("syrup bool unknown value: %v", b)
}

func syrupProtoSymbolVal(b []byte) (Symbol, error) {
	return Symbol(string(b)), nil
}

func syrupProtoStringVal(b []byte) (string, error) {
	return string(b), nil
}

func syrupProtoInt64Val(b []byte) (int64, *big.Int, error) {
	i, err := strconv.ParseInt(string(b), 10, 64)
	if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
		var ok bool
		bi := big.NewInt(0)
		bi, ok = bi.SetString(string(b), 10)
		if !ok {
			return 0, nil, errors.New("syrup: could not convert value to big int")
		} else {
			return 0, bi, nil
		}
	} else {
		return i, nil, err
	}
}

func syrupProtoFloat32Val(b []byte) (float32, error) {
	u := binary.BigEndian.Uint32(b)
	return math.Float32frombits(u), nil
}

func syrupProtoFloat64Val(b []byte) (float64, error) {
	u := binary.BigEndian.Uint64(b)
	return math.Float64frombits(u), nil
}
