// Code generated by "enumer -type=valueOrder -trimprefix=valueOrder -text -transform=kebab -output enums_valueorder.go"; DO NOT EDIT.

//
package imagine

import (
	"fmt"
)

const _valueOrderName = "linearstridepermutezipf"

var _valueOrderIndex = [...]uint8{0, 6, 12, 19, 23}

func (i valueOrder) String() string {
	if i < 0 || i >= valueOrder(len(_valueOrderIndex)-1) {
		return fmt.Sprintf("valueOrder(%d)", i)
	}
	return _valueOrderName[_valueOrderIndex[i]:_valueOrderIndex[i+1]]
}

var _valueOrderValues = []valueOrder{0, 1, 2, 3}

var _valueOrderNameToValueMap = map[string]valueOrder{
	_valueOrderName[0:6]:   0,
	_valueOrderName[6:12]:  1,
	_valueOrderName[12:19]: 2,
	_valueOrderName[19:23]: 3,
}

// valueOrderString retrieves an enum value from the enum constants string name.
// Throws an error if the param is not part of the enum.
func valueOrderString(s string) (valueOrder, error) {
	if val, ok := _valueOrderNameToValueMap[s]; ok {
		return val, nil
	}
	return 0, fmt.Errorf("%s does not belong to valueOrder values", s)
}

// valueOrderValues returns all values of the enum
func valueOrderValues() []valueOrder {
	return _valueOrderValues
}

// IsAvalueOrder returns "true" if the value is listed in the enum definition. "false" otherwise
func (i valueOrder) IsAvalueOrder() bool {
	for _, v := range _valueOrderValues {
		if i == v {
			return true
		}
	}
	return false
}

// MarshalText implements the encoding.TextMarshaler interface for valueOrder
func (i valueOrder) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface for valueOrder
func (i *valueOrder) UnmarshalText(text []byte) error {
	var err error
	*i, err = valueOrderString(string(text))
	return err
}