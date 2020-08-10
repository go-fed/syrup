package syrup

import (
	"fmt"
	"math/big"
	"strings"
)

type scanState uint8

const (
	scanFindToken scanState = iota
	scanTokenLen
	scanString
	scanFirstInt
	scanInt
	scanFloat64
	scanFloat32
	scanSymbol
	scanByteArr
)

type op uint8

const (
	noop op = iota
	valBoolop
	valByteArrOp
	valSymbolOp
	valStringOp
	valIntOp
	valFloat32Op
	valFloat64Op
	openListOp
	openDictOp
	openSetOp
	openRecordOp
	closeListOp
	closeDictOp
	closeSetOp
	closeRecordOp
)

type scanner struct {
	enc  *Encoding
	s    scanState
	buf  strings.Builder
	nlen uint64
}

// Process handles one byte of input at a time, processing the syrup encoding
// into a value.
//
// If 'oper' is a noop, the client code can proceed to call process again.
//
// If 'oper' is a 'val' op, the client code must obtain the accumulated value
// using the correct accessor.
//
// If 'oper' is an 'open' or 'close' op, the client code is recommended to
// create or finish populating an appropriate container.
func (s *scanner) Process(b byte) (oper op, err error) {
	var include bool
	next := s.s

	// 1. Determine State Transition
	//
	// Find out what the next state of our byte-interpretation (scanState)
	// is. Determine the high level operation that needs to happen, if any
	// need to. Determine whether to include the examined byte as a value
	// in our buffer: not all semantic bytes that are examined have value,
	// for example the prototypical 't' for true is both a semantic and
	// value byte, 'i' denoting an int is only semantically meaningful but
	// provides no value to the integer being described.
	switch s.s {
	case scanFindToken:
		next, oper, include, err = s.enc.mustFindToken(b)
	case scanTokenLen:
		next, oper, include, err = s.enc.scanTokenLen(b)
	case scanSymbol:
		next, oper, include = s.processLengthDeterminedType(valSymbolOp)
	case scanString:
		next, oper, include = s.processLengthDeterminedType(valStringOp)
	case scanByteArr:
		next, oper, include = s.processLengthDeterminedType(valByteArrOp)
	case scanInt:
		next, oper, include, err = s.enc.scanIntToken(b)
	case scanFirstInt:
		next, oper, include, err = s.enc.scanFirstIntToken(b)
	case scanFloat64:
		next, oper, include, err = s.enc.scanFloat64Token(b)
	case scanFloat32:
		next, oper, include, err = s.enc.scanFloat32Token(b)
	default:
		err = fmt.Errorf("syrup unknown scanstate: %d", s.s)
	}
	if err != nil {
		return
	}

	// 2. Include the bytes into the buffer if necessary.
	if include {
		// strings.Builder.WriteByte always returns 'nil'
		_ = s.buf.WriteByte(b)
	}
	// 3. In the special case of parsing a token-length, set our internal
	// buffer and length counters appropriately.
	//
	// Unfortunately this is a leak between the encoding and this generic
	// scanner.
	if s.s == scanTokenLen && next != scanTokenLen {
		if oper, err = s.processParsedLen(next); err != nil {
			return
		}
	}
	// 4. Finally, transition to the next state.
	s.s = next
	// 5. If a value-op was returned, it is up to the caller to ensure they
	// call one of the value functions, which has the side effect of
	// clearing the internal buffer.
	return
}

func (s *scanner) processParsedLen(next scanState) (oper op, err error) {
	oper, s.nlen, err = s.enc.parseLen(s.buf.String(), next)
	s.buf.Reset()
	return
}

func (s *scanner) processLengthDeterminedType(maybeOp op) (next scanState, oper op, include bool) {
	next = s.s
	include = true
	s.nlen--
	if s.nlen == 0 {
		next = scanFindToken
		oper = maybeOp
	}
	return
}

func (s *scanner) Bool() (bool, error) {
	b, err := s.enc.boolVal([]byte(s.buf.String()))
	s.buf.Reset()
	return b, err
}

func (s *scanner) Bytes() ([]byte, error) {
	b := []byte(s.buf.String())
	s.buf.Reset()
	return b, nil
}

func (s *scanner) Symbol() (Symbol, error) {
	sym, err := s.enc.symbolVal([]byte(s.buf.String()))
	s.buf.Reset()
	return sym, err
}

func (s *scanner) String() (string, error) {
	str, err := s.enc.stringVal([]byte(s.buf.String()))
	s.buf.Reset()
	return str, err
}

func (s *scanner) Int64() (int64, *big.Int, error) {
	i, b, err := s.enc.int64Val([]byte(s.buf.String()))
	s.buf.Reset()
	return i, b, err
}

func (s *scanner) Float32() (float32, error) {
	f, err := s.enc.float32Val([]byte(s.buf.String()))
	s.buf.Reset()
	return f, err
}

func (s *scanner) Float64() (float64, error) {
	f, err := s.enc.float64Val([]byte(s.buf.String()))
	s.buf.Reset()
	return f, err
}
