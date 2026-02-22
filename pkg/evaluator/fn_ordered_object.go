package evaluator

import (
	"bytes"
	"encoding/json"
)

type OrderedObject struct {
	Keys   []string
	Values map[string]interface{}
}

// Get retrieves a value by key.

func (o *OrderedObject) Get(key string) (interface{}, bool) {
	value, ok := o.Values[key]
	return value, ok
}

// MarshalJSON preserves key order during marshaling.

func (o *OrderedObject) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, key := range o.Keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		keyBytes, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		buf.Write(keyBytes)
		buf.WriteByte(':')
		valueBytes, err := json.Marshal(o.Values[key])
		if err != nil {
			return nil, err
		}
		buf.Write(valueBytes)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
