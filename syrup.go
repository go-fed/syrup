package syrup

import (
	"fmt"
	"io"
	"math/big"
	"reflect"
)

const (
	kSyrupStructTag = "syrup"
)

// NewEncoder creates a new syrup encoder using the specified encoding, and
// writes encoded values to 'w'.
func NewEncoder(enc *Encoding, w io.Writer) *Encoder {
	return &Encoder{enc: enc, w: w}
}

// NewDecoder creates a new syrup decoder using the specified encoding, decoding
// the byte stream provided by 'r'.
func NewDecoder(enc *Encoding, r io.Reader) *Decoder {
	return &Decoder{r: r, s: &scanner{enc: enc}}
}

// Encoder uses a specific syrup encoding to write encoded values.
type Encoder struct {
	enc *Encoding
	w   io.Writer
}

var typeOfByteSlice = reflect.TypeOf([]byte(nil))

var typeOfBigInt = reflect.TypeOf(big.NewInt(0))

// Encode writes the encoded value to the Encoder's writer. Syrup encodes
// privitive values into their Syrup counterparts. Slices are encoded as lists,
// maps are encoded as dictionaries, and structs are encoded as dictionaries
// using their public fields. Pointers are never encoded raw; they are
// dereferenced before encoding.
//
// For Symbols, Records, and Sets use the types provided by the syrup library as
// hints.
func (e *Encoder) Encode(v interface{}) error {
	var b []byte
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.String:
		if rv.Type() == typeOfSymbol {
			b = e.enc.fmtSymbol(rv.String())
		} else {
			b = e.enc.fmtString(rv.String())
		}
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		b = e.enc.fmtInt(rv.Int())
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		b = e.enc.fmtUint(rv.Uint())
	case reflect.Bool:
		b = e.enc.fmtBool(rv.Bool())
	case reflect.Float32:
		b = e.enc.fmtFloat32(float32(rv.Float()))
	case reflect.Float64:
		b = e.enc.fmtFloat64(rv.Float())
	case reflect.Slice:
		if rv.IsNil() {
			return fmt.Errorf("cannot encode nil pointer: %T", v)
		}
		if rv.Type() == typeOfByteSlice {
			b = e.enc.fmtBytes(rv.Bytes())
			break
			// b is used, break out
		} else if rv.Type() == typeOfSet {
			if err := e.write(e.enc.setOpen()); err != nil {
				return err
			}
			for idx := 0; idx < rv.Len(); idx++ {
				val := rv.Index(idx).Interface()
				if err := e.Encode(val); err != nil {
					return err
				}
			}
			return e.write(e.enc.setClose())
			// b is unused, return before leaving statement
		} else {
			if err := e.write(e.enc.listOpen()); err != nil {
				return err
			}
			for idx := 0; idx < rv.Len(); idx++ {
				val := rv.Index(idx).Interface()
				if err := e.Encode(val); err != nil {
					return err
				}
			}
			return e.write(e.enc.listClose())
			// b is unused, return before leaving statement
		}
	case reflect.Ptr:
		if rv.IsNil() {
			return fmt.Errorf("cannot encode nil pointer: %T", v)
		}
		if rv.Type() == typeOfBigInt {
			// Encode as big int
			b = e.enc.fmtBigInt(rv.Interface().(*big.Int))
		} else {
			return e.Encode(rv.Elem().Interface())
		}
		// b is unused, return before leaving statement
	case reflect.Map:
		if rv.IsNil() {
			return fmt.Errorf("cannot encode nil pointer: %T", v)
		}
		if err := e.write(e.enc.dictOpen()); err != nil {
			return err
		}
		iter := rv.MapRange()
		for iter.Next() {
			k := iter.Key()
			if err := e.Encode(k.Interface()); err != nil {
				return err
			}
			v := iter.Value()
			if err := e.Encode(v.Interface()); err != nil {
				return err
			}
		}
		return e.write(e.enc.dictClose())
		// b is unused, return before leaving statement
	case reflect.Struct:
		if rv.Type() == typeOfRecord {
			// Encode as record
			record := rv.Interface().(Record)
			if err := e.write(e.enc.recordOpen()); err != nil {
				return err
			}
			if err := e.Encode(record.Label); err != nil {
				return err
			}
			for _, val := range record.Values {
				if err := e.Encode(val); err != nil {
					return err
				}
			}
			return e.write(e.enc.recordClose())
		} else {
			// Encode as dictionary
			if err := e.write(e.enc.dictOpen()); err != nil {
				return err
			}
			n := rv.NumField()
			rvt := rv.Type()
			for i := 0; i < n; i++ {
				k := rvt.Field(i)
				if len(k.PkgPath) > 0 {
					// Skip unexported fields
					continue
				}
				name := k.Name
				if tname, ok := k.Tag.Lookup(kSyrupStructTag); ok {
					name = tname
				}
				if err := e.Encode(name); err != nil {
					return err
				}
				v := rv.Field(i)
				if err := e.Encode(v.Interface()); err != nil {
					return err
				}
			}
			return e.write(e.enc.dictClose())
		}
		// b is unused, return before leaving statement
	default:
		return fmt.Errorf("unknown type: %T", v)
	}
	return e.write(b)
}

func (e *Encoder) write(b []byte) error {
	n, err := e.w.Write(b)
	if err != nil {
		return err
	} else if n != len(b) {
		return fmt.Errorf("wrote %d of %d bytes", n, len(b))
	}
	return nil
}

type InvalidTypeError struct {
	Value  string
	Type   reflect.Type
	Offset uint64
}

func (e *InvalidTypeError) Error() string {
	return fmt.Sprintf("syrup: cannot decode %s into Go value of type %s at byte offset %d", e.Value, e.Type, e.Offset)
}

type InvalidDecodeError struct {
	Type reflect.Type
}

func (e *InvalidDecodeError) Error() string {
	if e.Type == nil {
		return "syrup: decode given nil"
	}

	if e.Type.Kind() != reflect.Ptr {
		return "syrup: decode given non-pointer " + e.Type.String()
	}
	return "syrup: decode given nil " + e.Type.String()
}

type Decoder struct {
	r io.Reader
	s *scanner
	n uint64
}

func (d *Decoder) Decode(v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &InvalidDecodeError{reflect.TypeOf(v)}
	}
	_, err := d.run(reflect.ValueOf(v))
	if err == io.EOF {
		err = nil
	}
	return err
}

func (d *Decoder) run(v reflect.Value) (last op, err error) {
	nb := make([]byte, 1)
	stop := false
	for err == nil && !stop {
		var n int
		n, err = d.r.Read(nb)
		if n != 1 && err == nil {
			err = fmt.Errorf("syrup read %d bytes instead of 1 byte", n)
		}
		if n == 1 {
			var err2 error
			last, err2 = d.s.Process(nb[0])
			if err2 != nil {
				return last, err2
			}
			var err3 error
			stop, err3 = d.handleOp(v, last)
			if err3 != nil {
				return last, err3
			}
		}
	}
	return last, err
}

func (d *Decoder) handleOp(v reflect.Value, oper op) (stop bool, err error) {
	if !v.IsValid() {
		// Silently skip some values, similar to encoding/json.
		//
		// When a fixed array runs out of space, we keep similar
		// behavior to encoding/json and silently drop the tail end
		// of things.
		d.n++
		return
	}
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	stop = true
	switch oper {
	case noop:
		stop = false
		d.n++
		break
	case valBoolop:
		b := false
		b, err = d.s.Bool()
		if err != nil {
			return
		}
		err = d.storeBool(v, b)
		d.n++
	case valByteArrOp:
		var b []byte
		b, err = d.s.Bytes()
		if err != nil {
			return
		}
		err = d.storeByteArr(v, b)
		d.n++
	case valSymbolOp:
		var s Symbol
		s, err = d.s.Symbol()
		if err != nil {
			return
		}
		err = d.storeSymbol(v, s)
		d.n++
	case valStringOp:
		var s string
		s, err = d.s.String()
		if err != nil {
			return
		}
		err = d.storeString(v, s)
		d.n++
	case valIntOp:
		var i int64
		var bi *big.Int
		i, bi, err = d.s.Int64()
		if err != nil {
			return
		}
		if bi != nil {
			err = d.storeBigInt(v, bi)
		} else {
			err = d.storeInt(v, i)
		}
		d.n++
	case valFloat32Op:
		var f float32
		f, err = d.s.Float32()
		if err != nil {
			return
		}
		err = d.storeFloat32(v, f)
		d.n++
	case valFloat64Op:
		var f float64
		f, err = d.s.Float64()
		if err != nil {
			return
		}
		err = d.storeFloat64(v, f)
		d.n++
	case openListOp:
		err = d.recurCreatingSliceOrArray(v, oper, closeListOp, "list")
	case openDictOp:
		pv := v
		if v.Kind() == reflect.Ptr {
			pv = v.Elem()
		}
		var m structMetadata
		var mt reflect.Type
		switch pv.Kind() {
		case reflect.Interface:
			// Non-reflective shortcut
			err = d.interfaceDict(pv, oper)
			return
		case reflect.Map:
			// Maps must have interface or string key
			mt = pv.Type()
			if mt.Key().Kind() != reflect.Interface &&
				mt.Key().Kind() != reflect.String {
				err = &InvalidTypeError{Value: "dict", Type: pv.Type(), Offset: d.n}
				return
			}
			if pv.IsNil() {
				pv.Set(reflect.MakeMap(mt))
			}
		case reflect.Struct:
			m = buildCachedMetadata(pv.Type())
			if v.Kind() == reflect.Ptr && v.IsNil() {
				pv.Set(reflect.New(pv.Type()).Elem())
			}
		default:
			err = &InvalidTypeError{Value: "dict", Type: pv.Type(), Offset: d.n}
		}
		d.n++
		var last op
		for last != closeDictOp {
			if pv.Kind() == reflect.Map {
				key := reflect.New(mt.Key()).Elem()
				if last, err = d.run(key); err != nil {
					return
				}
				if last != closeDictOp {
					val := reflect.New(mt.Elem()).Elem()
					if last, err = d.run(val); err != nil {
						return
					}
					pv.SetMapIndex(key, val)
				}
			} else { // reflect.Struct
				// Only strings supported as keys
				var skey string
				key := reflect.ValueOf(&skey)
				if last, err = d.run(key); err != nil {
					return
				}
				if last != closeDictOp {
					var val reflect.Value
					if idx, ok := m.fieldNamesIndex[skey]; ok {
						val = pv.Field(m.fields[idx].fieldIdx)
						if !val.CanSet() {
							err = fmt.Errorf("syrup: cannot set field %s when processing dict at byte offset %d", skey, d.n)
							return
						}
						if val.Kind() == reflect.Ptr && val.IsNil() {
							val.Set(reflect.New(val.Type().Elem()))
							val = val.Elem()
						}
					}
					if last, err = d.run(val); err != nil {
						return
					}
				}
			}
		}
	case openSetOp:
		err = d.recurCreatingSliceOrArray(v, oper, closeSetOp, "set")
	case openRecordOp:
		pv := v
		if v.Kind() == reflect.Ptr {
			pv = v.Elem()
		}
		if pv.Type() != typeOfRecord && pv.Kind() != reflect.Interface {
			err = &InvalidTypeError{Value: "record", Type: pv.Type(), Offset: d.n}
			return
		}
		var r Record
		var last op
		if last, err = d.run(reflect.ValueOf(&r.Label)); err != nil {
			return
		}
		for last != closeRecordOp {
			var ele interface{}
			if last, err = d.run(reflect.ValueOf(&ele)); err != nil {
				return
			}
			r.Values = append(r.Values, ele)
		}
		// We have 1 extra element
		if len(r.Values) > 0 {
			r.Values = r.Values[:len(r.Values)-1]
		}
		pv.Set(reflect.ValueOf(r))
	case closeListOp:
		break
	case closeDictOp:
		break
	case closeSetOp:
		break
	case closeRecordOp:
		break
	default:
		err = fmt.Errorf("syrup unknown op: %v", oper)
	}
	return
}

func (d *Decoder) interfaceDict(v reflect.Value, oper op) (err error) {
	vals := make(map[interface{}]interface{}, 0)
	var last op
	for last != closeDictOp {
		var k interface{}
		if last, err = d.run(reflect.ValueOf(&k)); err != nil {
			return
		}
		if last != closeDictOp {
			var vi interface{}
			if last, err = d.run(reflect.ValueOf(&vi)); err != nil {
				return
			}
			vals[k] = vi
		}
	}
	v.Set(reflect.ValueOf(vals))
	return
}

func (d *Decoder) recurCreatingSliceOrArray(v reflect.Value, oper op, stop op, errHint string) (err error) {
	pv := v
	if v.Kind() == reflect.Ptr {
		pv = v.Elem()
	}
	switch pv.Kind() {
	case reflect.Interface:
		// Non-reflective shortcut
		err = d.interfaceSlice(pv, oper)
		return
	case reflect.Array, reflect.Slice:
		break
	default:
		err = &InvalidTypeError{Value: errHint, Type: pv.Type(), Offset: d.n}
		return
	}
	d.n++
	i := 0
	var last op
	for last != stop {
		// When a slice, grow its length by 1 and capacity by a
		// factor of half.
		if pv.Kind() == reflect.Slice {
			if i >= pv.Cap() {
				capt := pv.Cap() + pv.Cap()/2
				if capt < 4 {
					capt = 4
				}
				next := reflect.MakeSlice(pv.Type(), pv.Len(), capt)
				reflect.Copy(next, pv)
				pv.Set(next)
			}
			if i >= pv.Len() {
				pv.SetLen(i + 1)
			}
		}
		// Recursively populate the list.
		if i < v.Len() {
			if last, err = d.run(pv.Index(i)); err != nil {
				return
			}
		} else {
			// Ran out of fixed array.
			if last, err = d.run(reflect.Value{}); err != nil {
				return
			}
		}
		i++
	}
	// We have 1 more element than necessary -- the iteration that
	// returns clostListOp still had created a spot in the
	// array/slice.
	i--
	if pv.Kind() == reflect.Array {
		// Pad the rest of the array with zero values.
		zero := reflect.Zero(pv.Type().Elem())
		for ; i < pv.Len(); i++ {
			pv.Index(i).Set(zero)
		}
	} else if i == 0 {
		// Zero slice
		pv.Set(reflect.MakeSlice(pv.Type(), 0, 0))
	} else {
		// Shrinkwrap -- we had 1 more element than necessary.
		pv.SetLen(i)
	}
	return
}

func (d *Decoder) interfaceSlice(v reflect.Value, oper op) (err error) {
	vals := make([]interface{}, 0)
	var last op
	for last != closeListOp {
		var n interface{}
		if last, err = d.run(reflect.ValueOf(&n)); err != nil {
			return
		}
		vals = append(vals, n)
	}
	// We have 1 extra element.
	v.Set(reflect.ValueOf(vals[:len(vals)-1]))
	return
}

func (d *Decoder) storeBool(v reflect.Value, b bool) error {
	var err error
	switch v.Kind() {
	case reflect.Bool:
		v.SetBool(b)
	case reflect.Interface:
		if v.NumMethod() == 0 {
			v.Set(reflect.ValueOf(b))
		} else {
			err = &InvalidTypeError{Value: "bool", Type: v.Type(), Offset: d.n}
		}
	default:
		err = &InvalidTypeError{Value: "bool", Type: v.Type(), Offset: d.n}
	}
	return err
}

func (d *Decoder) storeByteArr(v reflect.Value, b []byte) error {
	var err error
	switch v.Kind() {
	case reflect.String:
		v.SetString(string(b))
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			err = &InvalidTypeError{Value: "bytestring", Type: v.Type(), Offset: d.n}
		} else {
			v.SetBytes(b)
		}
	case reflect.Interface:
		if v.NumMethod() == 0 {
			v.Set(reflect.ValueOf(b))
		} else {
			err = &InvalidTypeError{Value: "bytestring", Type: v.Type(), Offset: d.n}
		}
	default:
		err = &InvalidTypeError{Value: "bytestring", Type: v.Type(), Offset: d.n}
	}
	return err
}

func (d *Decoder) storeSymbol(v reflect.Value, s Symbol) error {
	var err error
	switch v.Kind() {
	case reflect.String:
		if v.Type() == typeOfSymbol {
			v.SetString(string(s))
		} else {
			err = &InvalidTypeError{Value: "symbol", Type: v.Type(), Offset: d.n}
		}
	case reflect.Ptr:
		return d.storeSymbol(v.Elem(), s)
	case reflect.Interface:
		if v.NumMethod() == 0 {
			v.Set(reflect.ValueOf(s))
		} else {
			err = &InvalidTypeError{Value: "symbol", Type: v.Type(), Offset: d.n}
		}
	default:
		err = &InvalidTypeError{Value: "symbol", Type: v.Type(), Offset: d.n}
	}
	return err
}

func (d *Decoder) storeString(v reflect.Value, s string) error {
	var err error
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			err = &InvalidTypeError{Value: "string", Type: v.Type(), Offset: d.n}
		} else {
			v.SetBytes([]byte(s))
		}
	case reflect.Interface:
		if v.NumMethod() == 0 {
			v.Set(reflect.ValueOf(s))
		} else {
			err = &InvalidTypeError{Value: "string", Type: v.Type(), Offset: d.n}
		}
	default:
		err = &InvalidTypeError{Value: "string", Type: v.Type(), Offset: d.n}
	}
	return err
}

func (d *Decoder) storeBigInt(v reflect.Value, i *big.Int) error {
	var err error
	switch v.Kind() {
	case reflect.Ptr:
		if v.Type() == typeOfBigInt {
			v.Set(reflect.ValueOf(i))
		} else {
			err = &InvalidTypeError{Value: "big integer", Type: v.Type(), Offset: d.n}
		}
	case reflect.Interface:
		if v.NumMethod() == 0 {
			v.Set(reflect.ValueOf(i))
		} else {
			err = &InvalidTypeError{Value: "big integer", Type: v.Type(), Offset: d.n}
		}
	default:
		err = &InvalidTypeError{Value: "big integer", Type: v.Type(), Offset: d.n}
	}
	return err
}

func (d *Decoder) storeInt(v reflect.Value, i int64) error {
	var err error
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.OverflowInt(i) {
			err = &InvalidTypeError{Value: "integer overflow", Type: v.Type(), Offset: d.n}
		} else {
			v.SetInt(i)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v.OverflowUint(uint64(i)) {
			err = &InvalidTypeError{Value: "unsigned integer overflow", Type: v.Type(), Offset: d.n}
		} else {
			v.SetUint(uint64(i))
		}
	case reflect.Interface:
		if v.NumMethod() == 0 {
			v.Set(reflect.ValueOf(i))
		} else {
			err = &InvalidTypeError{Value: "integer", Type: v.Type(), Offset: d.n}
		}
	default:
		err = &InvalidTypeError{Value: "integer", Type: v.Type(), Offset: d.n}
	}
	return err
}

func (d *Decoder) storeFloat32(v reflect.Value, f float32) error {
	var err error
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		if v.OverflowFloat(float64(f)) {
			err = &InvalidTypeError{Value: "single precision float overflow", Type: v.Type(), Offset: d.n}
		} else {
			v.SetFloat(float64(f))
		}
	case reflect.Interface:
		if v.NumMethod() == 0 {
			v.Set(reflect.ValueOf(f))
		} else {
			err = &InvalidTypeError{Value: "single precision float", Type: v.Type(), Offset: d.n}
		}
	default:
		err = &InvalidTypeError{Value: "single precision float", Type: v.Type(), Offset: d.n}
	}
	return err
}

func (d *Decoder) storeFloat64(v reflect.Value, f float64) error {
	var err error
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		if v.OverflowFloat(f) {
			err = &InvalidTypeError{Value: "double precision float overflow", Type: v.Type(), Offset: d.n}
		} else {
			v.SetFloat(f)
		}
	case reflect.Interface:
		if v.NumMethod() == 0 {
			v.Set(reflect.ValueOf(f))
		} else {
			err = &InvalidTypeError{Value: "double precision float", Type: v.Type(), Offset: d.n}
		}
	default:
		err = &InvalidTypeError{Value: "double precision float", Type: v.Type(), Offset: d.n}
	}
	return err
}
