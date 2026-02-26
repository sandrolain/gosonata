package evaluator

import (
	"encoding/json"
	"strconv"
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
	buf := acquireBuf()
	defer releaseBuf(buf)
	buf.WriteByte('{')
	for i, key := range o.Keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		// OPT-05: strconv.Quote è equivalente a json.Marshal per le stringhe e non alloca []byte+error
		buf.WriteString(strconv.Quote(key))
		buf.WriteByte(':')
		valueBytes, err := json.Marshal(o.Values[key])
		if err != nil {
			return nil, err
		}
		buf.Write(valueBytes)
	}
	buf.WriteByte('}')
	// buf.Bytes() points into the pooled buffer's memory; copy before releasing.
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result, nil
}
