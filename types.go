package syrup

import (
	"reflect"
)

// Symbol is a syrup symbol.
type Symbol string

// Set is a syrup set, encoded as such. It is up to users of the library to
// enforce that any Set being serialized or desearialized has unique elements.
// The syrup library does not determine if elements are duplicates.
type Set []interface{}

// Record is a syrup record. It contains a single label and zero or more values.
type Record struct {
	Label  interface{}
	Values []interface{}
}

var typeOfSymbol = reflect.TypeOf(Symbol(""))

var typeOfSet = reflect.TypeOf(Set([]interface{}{}))

var typeOfRecord = reflect.TypeOf(Record{})
