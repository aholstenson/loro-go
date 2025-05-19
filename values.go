package loro

import "fmt"

type simpleValue struct {
	value LoroValue
}

func (v simpleValue) AsLoroValue() LoroValue {
	return v.value
}

// AsStringValue converts a string to a LoroValueLike which can be used in most
// places that expect a value.
func AsStringValue(v string) LoroValueLike {
	return simpleValue{
		value: LoroValueString{
			Value: v,
		},
	}
}

// AsNilValue converts a null value to a LoroValueLike which can be used in most
// places that expect a value.
func AsNilValue() LoroValueLike {
	return simpleValue{
		value: LoroValueNull{},
	}
}

// AsBoolValue converts a bool to a LoroValueLike which can be used in most
// places that expect a value.
func AsBoolValue(v bool) LoroValueLike {
	return simpleValue{
		value: LoroValueBool{
			Value: v,
		},
	}
}

// AsFloat64Value converts a float64 to a LoroValueLike which can be used in most
// places that expect a value.
func AsFloat64Value(v float64) LoroValueLike {
	return simpleValue{
		value: LoroValueDouble{
			Value: v,
		},
	}
}

// AsInt64Value converts an int64 to a LoroValueLike which can be used in most
// places that expect a value.
func AsInt64Value(v int64) LoroValueLike {
	return simpleValue{
		value: LoroValueI64{
			Value: v,
		},
	}
}

// AsListValue converts a list of LoroValueLike to a LoroValueLike which can be
// used in most places that expect a value.
func AsListValue(v []LoroValueLike) LoroValueLike {
	values := make([]LoroValue, len(v))
	for i, value := range v {
		values[i] = value.AsLoroValue()
	}
	return simpleValue{
		value: LoroValueList{
			Value: values,
		},
	}
}

// AsListValueFromVec converts a vector of LoroValue to a LoroValueLike which
// can be used in most places that expect a value.
func AsListValueFromVec(v []LoroValue) LoroValueLike {
	return simpleValue{
		value: LoroValueList{
			Value: v,
		},
	}
}

// AsListValueFromVecOfAny converts a vector of any to a LoroValueLike which
// can be used in most places that expect a value.
func AsListValueFromVecOfAny(v []any) (LoroValueLike, error) {
	values := make([]LoroValue, len(v))
	for i, value := range v {
		value, err := AsValue(value)
		if err != nil {
			return nil, err
		}

		values[i] = value.AsLoroValue()
	}

	return AsListValueFromVec(values), nil
}

// AsMapValue converts a map of string to LoroValueLike to a LoroValueLike which
// can be used in most places that expect a value.
func AsMapValue(v map[string]LoroValueLike) LoroValueLike {
	values := make(map[string]LoroValue)
	for k, v := range v {
		values[k] = v.AsLoroValue()
	}
	return simpleValue{
		value: LoroValueMap{
			Value: values,
		},
	}
}

// AsMapValueFromVec converts a map of string to LoroValue to a LoroValueLike
// which can be used in most places that expect a value.
func AsMapValueFromVec(v map[string]LoroValue) LoroValueLike {
	return simpleValue{
		value: LoroValueMap{
			Value: v,
		},
	}
}

// AsMapValueFromVecOfAny converts a map of string to any to a LoroValueLike
// which can be used in most places that expect a value.
func AsMapValueFromVecOfAny(v map[string]any) (LoroValueLike, error) {
	values := make(map[string]LoroValue)

	for k, v := range v {
		value, err := AsValue(v)
		if err != nil {
			return nil, err
		}
		values[k] = value.AsLoroValue()
	}

	return AsMapValueFromVec(values), nil
}

// AsValue converts an any to a LoroValueLike which can be used in most
// places that expect a value.
func AsValue(v any) (LoroValueLike, error) {
	if v == nil {
		return AsNilValue(), nil
	}

	switch v := v.(type) {
	case string:
		return AsStringValue(v), nil
	case bool:
		return AsBoolValue(v), nil
	case float64:
		return AsFloat64Value(v), nil
	case int64:
		return AsInt64Value(v), nil
	case []any:
		return AsListValueFromVecOfAny(v)
	case map[string]any:
		return AsMapValueFromVecOfAny(v)
	}

	return nil, fmt.Errorf("unsupported value type: %T", v)
}

// GetValue takes ValueOrContainer and returns the LoroValue if it is a value.
func GetValue(v **ValueOrContainer) (LoroValue, bool) {
	if v == nil {
		return nil, false
	}

	v0 := *v
	if v0 == nil {
		return nil, false
	}

	loroValue := v0.AsValue()
	if loroValue == nil {
		return nil, false
	}

	return *loroValue, true
}

// IsValueExplicitlyNil checks if a pointer to a ValueOrContainer is explicitly
// nil, meaning it has a LoroValueNull value.
func IsValueExplicitlyNil(v **ValueOrContainer) bool {
	value, ok := GetValue(v)
	if !ok {
		return false
	}

	if _, ok := value.(LoroValueNull); ok {
		return true
	}

	return false
}

// GetStringValue takes a pointer to a ValueOrContainer and returns the string
// value if it is a string.
func GetStringValue(v **ValueOrContainer) (string, bool) {
	value, ok := GetValue(v)
	if !ok {
		return "", false
	}

	if str, ok := value.(LoroValueString); ok {
		return str.Value, true
	}

	return "", false
}

// GetBoolValue takes a pointer to a ValueOrContainer and returns the bool
// value if it is a bool.
func GetBoolValue(v **ValueOrContainer) (bool, bool) {
	value, ok := GetValue(v)
	if !ok {
		return false, false
	}

	if bool, ok := value.(LoroValueBool); ok {
		return bool.Value, true
	}

	return false, false
}

// GetFloat64Value takes a pointer to a ValueOrContainer and returns the float64
// value if it is a float64.
func GetFloat64Value(v **ValueOrContainer) (float64, bool) {
	value, ok := GetValue(v)
	if !ok {
		return 0, false
	}

	if float, ok := value.(LoroValueDouble); ok {
		return float.Value, true
	}

	return 0, false
}

// GetInt64Value takes a pointer to a ValueOrContainer and returns the int64
// value if it is an int64.
func GetInt64Value(v **ValueOrContainer) (int64, bool) {
	value, ok := GetValue(v)
	if !ok {
		return 0, false
	}

	if int, ok := value.(LoroValueI64); ok {
		return int.Value, true
	}

	return 0, false
}

// GetListValue takes a pointer to a ValueOrContainer and returns the list
// value if it is a list.
func GetListValue(v **ValueOrContainer) ([]LoroValue, bool) {
	value, ok := GetValue(v)
	if !ok {
		return nil, false
	}

	if list, ok := value.(LoroValueList); ok {
		return list.Value, true
	}

	return nil, false
}

// GetListValueOfAny takes a pointer to a ValueOrContainer and returns a slice
// of any if it is a list.
func GetListValueOfAny(v **ValueOrContainer) ([]any, bool) {
	value, ok := GetValue(v)
	if !ok {
		return nil, false
	}

	if list, ok := value.(LoroValueList); ok {
		values := make([]any, len(list.Value))
		for i, v := range list.Value {
			v, ok := GetBasicValueAsAny(v)
			if !ok {
				return nil, false
			}

			values[i] = v
		}

		return values, true
	}

	return nil, false
}

// GetMapValue takes a pointer to a ValueOrContainer and returns the map
// value if it is a map.
func GetMapValue(v **ValueOrContainer) (map[string]LoroValue, bool) {
	value, ok := GetValue(v)
	if !ok {
		return nil, false
	}

	if mapValue, ok := value.(LoroValueMap); ok {
		return mapValue.Value, true
	}

	return nil, false
}

// GetMapValueOfAny takes a pointer to a ValueOrContainer and returns a map
// of string to any if it is a map.
func GetMapValueOfAny(v **ValueOrContainer) (map[string]any, bool) {
	value, ok := GetValue(v)
	if !ok {
		return nil, false
	}

	if mapValue, ok := value.(LoroValueMap); ok {
		values := make(map[string]any)
		for k, v := range mapValue.Value {
			v, ok := GetBasicValueAsAny(v)
			if !ok {
				return nil, false
			}

			values[k] = v
		}

		return values, true
	}

	return nil, false
}

// GetAnyValue takes a pointer to a ValueOrContainer and returns the value
// as an any.
func GetAnyValue(v **ValueOrContainer) (any, bool) {
	value, ok := GetValue(v)
	if !ok {
		return nil, false
	}

	return GetBasicValueAsAny(value)
}

// GetBasicValueAsAny takes a LoroValue and returns the value as an any.
func GetBasicValueAsAny(value LoroValue) (any, bool) {
	switch value := value.(type) {
	case LoroValueNull:
		return nil, true
	case LoroValueString:
		return value.Value, true
	case LoroValueBool:
		return value.Value, true
	case LoroValueDouble:
		return value.Value, true
	case LoroValueI64:
		return value.Value, true
	case LoroValueList:
		values := make([]any, len(value.Value))

		for i, v := range value.Value {
			v, ok := GetBasicValueAsAny(v)
			if !ok {
				return nil, false
			}

			values[i] = v
		}

		return values, true
	case LoroValueMap:
		values := make(map[string]any)

		for k, v := range value.Value {
			v, ok := GetBasicValueAsAny(v)
			if !ok {
				return nil, false
			}

			values[k] = v
		}

		return values, true
	}

	return nil, false
}

// GetLoroCounterContainer takes a pointer to a ValueOrContainer and returns the
// LoroCounter value if it is a LoroCounter.
func GetLoroCounterContainer(v **ValueOrContainer) (*LoroCounter, bool) {
	if v == nil {
		return nil, false
	}

	v0 := *v
	if v0 == nil {
		return nil, false
	}

	loroCounter := v0.AsLoroCounter()
	if loroCounter == nil {
		return nil, false
	}

	loroCounter0 := *loroCounter
	if loroCounter0 == nil {
		return nil, false
	}

	return loroCounter0, true
}

// GetLoroMapContainer takes a pointer to a ValueOrContainer and returns the
// LoroMap value if it is a LoroMap.
func GetLoroMapContainer(v **ValueOrContainer) (*LoroMap, bool) {
	if v == nil {
		return nil, false
	}

	v0 := *v
	if v0 == nil {
		return nil, false
	}

	loroMap := v0.AsLoroMap()
	if loroMap == nil {
		return nil, false
	}

	loroMap0 := *loroMap
	if loroMap0 == nil {
		return nil, false
	}

	return loroMap0, true
}

// GetLoroListContainer takes a pointer to a ValueOrContainer and returns the
// LoroList value if it is a LoroList.
func GetLoroListContainer(v **ValueOrContainer) (*LoroList, bool) {
	if v == nil {
		return nil, false
	}

	v0 := *v
	if v0 == nil {
		return nil, false
	}

	loroList := v0.AsLoroList()
	if loroList == nil {
		return nil, false
	}

	loroList0 := *loroList
	if loroList0 == nil {
		return nil, false
	}

	return loroList0, true
}

// GetLoroMovableListContainer takes a pointer to a ValueOrContainer and returns the
// LoroMovableList value if it is a LoroMovableList.
func GetLoroMovableListContainer(v **ValueOrContainer) (*LoroMovableList, bool) {
	if v == nil {
		return nil, false
	}

	v0 := *v
	if v0 == nil {
		return nil, false
	}

	loroMovableList := v0.AsLoroMovableList()
	if loroMovableList == nil {
		return nil, false
	}

	loroMovableList0 := *loroMovableList
	if loroMovableList0 == nil {
		return nil, false
	}

	return loroMovableList0, true
}

// GetLoroTextContainer takes a pointer to a ValueOrContainer and returns the
// LoroText value if it is a LoroText.
func GetLoroTextContainer(v **ValueOrContainer) (*LoroText, bool) {
	if v == nil {
		return nil, false
	}

	v0 := *v
	if v0 == nil {
		return nil, false
	}

	loroText := v0.AsLoroText()
	if loroText == nil {
		return nil, false
	}

	loroText0 := *loroText
	if loroText0 == nil {
		return nil, false
	}

	return loroText0, true
}

// GetLoroTreeContainer takes a pointer to a ValueOrContainer and returns the
// LoroTree value if it is a LoroTree.
func GetLoroTreeContainer(v **ValueOrContainer) (*LoroTree, bool) {
	if v == nil {
		return nil, false
	}

	v0 := *v
	if v0 == nil {
		return nil, false
	}

	loroTree := v0.AsLoroTree()
	if loroTree == nil {
		return nil, false
	}

	loroTree0 := *loroTree
	if loroTree0 == nil {
		return nil, false
	}

	return loroTree0, true
}
