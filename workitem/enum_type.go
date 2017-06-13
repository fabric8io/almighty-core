package workitem

import (
	"fmt"
	"reflect"

	"github.com/fabric8io/almighty-core/convert"
)

type EnumType struct {
	SimpleType
	BaseType SimpleType
	Values   []interface{}
}

// Ensure EnumType implements the Equaler interface
var _ convert.Equaler = EnumType{}
var _ convert.Equaler = (*EnumType)(nil)

// Equal returns true if two EnumType objects are equal; otherwise false is returned.
func (t EnumType) Equal(u convert.Equaler) bool {
	other, ok := u.(EnumType)
	if !ok {
		return false
	}
	if !t.SimpleType.Equal(other.SimpleType) {
		return false
	}
	if !t.BaseType.Equal(other.BaseType) {
		return false
	}
	return reflect.DeepEqual(t.Values, other.Values)
}

func (fieldType EnumType) ConvertToModel(value interface{}) (interface{}, error) {
	converted, err := fieldType.BaseType.ConvertToModel(value)
	if err != nil {
		return nil, fmt.Errorf("error converting enum value: %s", err.Error())
	}

	if !contains(fieldType.Values, converted) {
		return nil, fmt.Errorf("not an enum value: %v", value)
	}
	return converted, nil
}

func contains(a []interface{}, v interface{}) bool {
	for _, element := range a {
		if element == v {
			return true
		}
	}
	return false
}

func (fieldType EnumType) ConvertFromModel(value interface{}) (interface{}, error) {
	converted, err := fieldType.BaseType.ConvertToModel(value)
	if err != nil {
		return nil, fmt.Errorf("error converting enum value: %s", err.Error())
	}
	return converted, nil
}
