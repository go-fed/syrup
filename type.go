package syrup

import (
	"reflect"
	"sync"
)

type structMetadata struct {
	fields          []field
	fieldNamesIndex map[string]int
}

type field struct {
	name     string
	fieldIdx int
	t        reflect.Type
}

func buildMetadata(t reflect.Type) (m structMetadata) {
	m.fieldNamesIndex = make(map[string]int, 0)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		public := f.PkgPath == ""
		if !public {
			continue
		}
		m.fieldNamesIndex[f.Name] = len(m.fields)
		m.fields = append(m.fields, field{
			name:     f.Name,
			fieldIdx: i,
			t:        f.Type,
		})
	}
	return m
}

// map[reflect.Type]structMetadata
var metadataCache sync.Map

func buildCachedMetadata(t reflect.Type) (m structMetadata) {
	if i, ok := metadataCache.Load(t); ok {
		m = i.(structMetadata)
		return
	}
	i, _ := metadataCache.LoadOrStore(t, buildMetadata(t))
	m = i.(structMetadata)
	return
}
