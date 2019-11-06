// Package gval provides a generic expression language.
// All functions, infix and prefix operators can be replaced by composing languages into a new one.
//
// The package contains concrete expression languages for common application in text, arithmetic, propositional logic and so on.
// They can be used as basis for a custom expression language or to evaluate expressions directly.
package gval

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"text/scanner"
	"time"
)

//Evaluate given parameter with given expression in gval full language
func Evaluate(expression string, parameter interface{}, opts ...Language) (interface{}, error) {
	l := full
	if len(opts) > 0 {
		l = NewLanguage(append([]Language{l}, opts...)...)
	}
	return l.Evaluate(expression, parameter)
}

// Full is the union of Arithmetic, Bitmask, Text, PropositionalLogic, and Json
// 		Operator in: a in b is true iff value a is an element of array b
// 		Operator ??: a ?? b returns a if a is not false or nil, otherwise n
// 		Operator ?: a ? b : c returns b if bool a is true, otherwise b
//
// Function Date: Date(a) parses string a. a must match RFC3339, ISO8601, ruby date, or unix date
func Full(extensions ...Language) Language {
	if len(extensions) == 0 {
		return full
	}
	return NewLanguage(append([]Language{full}, extensions...)...)
}

// Arithmetic contains base, plus(+), minus(-), divide(/), power(**), negative(-)
// and numerical order (<=,<,>,>=)
//
// Arithmetic operators expect float64 operands.
// Called with unfitting input, they try to convert the input to float64.
// They can parse strings and convert any type of int or float.
func Arithmetic() Language {
	return arithmetic
}

// Bitmask contains base, bitwise and(&), bitwise or(|) and bitwise not(^).
//
// Bitmask operators expect float64 operands.
// Called with unfitting input they try to convert the input to float64.
// They can parse strings and convert any type of int or float.
func Bitmask() Language {
	return bitmask
}

// Text contains base, lexical order on strings (<=,<,>,>=),
// regex match (=~) and regex not match (!~)
func Text() Language {
	return text
}

// PropositionalLogic contains base, not(!), and (&&), or (||) and Base.
//
// Propositional operator expect bool operands.
// Called with unfitting input they try to convert the input to bool.
// Numbers other than 0 and the strings "TRUE" and "true" are interpreted as true.
// 0 and the strings "FALSE" and "false" are interpreted as false.
func PropositionalLogic() Language {
	return propositionalLogic
}

// JSON contains json objects ({string:expression,...})
// and json arrays ([expression, ...])
func JSON() Language {
	return ljson
}

// Base contains equal (==) and not equal (!=), perentheses and general support for variables, constants and functions
// It contains true, false, (floating point) number, string  ("" or ``) and char ('') constants
func Base() Language {
	return base
}

var full = NewLanguage(arithmetic, bitmask, text, propositionalLogic, ljson,

	InfixOperator("in", inArray),

	InfixShortCircuit("??", func(a interface{}) (interface{}, bool) {
		return a, a != false && a != nil
	}),
	InfixOperator("??", func(a, b interface{}) (interface{}, error) {
		if a == false || a == nil {
			return b, nil
		}
		return a, nil
	}),

	PostfixOperator("?", parseIf),

	Function("date", func(arguments ...interface{}) (interface{}, error) {
		if len(arguments) != 1 {
			return nil, fmt.Errorf("date() expects exactly one string argument")
		}
		s, ok := arguments[0].(string)
		if !ok {
			return nil, fmt.Errorf("date() expects exactly one string argument")
		}
		for _, format := range [...]string{
			time.ANSIC,
			time.UnixDate,
			time.RubyDate,
			time.Kitchen,
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02",                         // RFC 3339
			"2006-01-02 15:04",                   // RFC 3339 with minutes
			"2006-01-02 15:04:05",                // RFC 3339 with seconds
			"2006-01-02 15:04:05-07:00",          // RFC 3339 with seconds and timezone
			"2006-01-02T15Z0700",                 // ISO8601 with hour
			"2006-01-02T15:04Z0700",              // ISO8601 with minutes
			"2006-01-02T15:04:05Z0700",           // ISO8601 with seconds
			"2006-01-02T15:04:05.999999999Z0700", // ISO8601 with nanoseconds
		} {
			ret, err := time.ParseInLocation(format, s, time.Local)
			if err == nil {
				return ret, nil
			}
		}
		return nil, fmt.Errorf("date() could not parse %s", s)
	}),
)

var ljson = NewLanguage(
	PrefixExtension('[', parseJSONArray),
	PrefixExtension('{', parseJSONObject),
)

var arithmetic = NewLanguage(
	InfixNumberOperator("+", func(a, b float64) (interface{}, error) { return a + b, nil }),
	InfixNumberOperator("-", func(a, b float64) (interface{}, error) { return a - b, nil }),
	InfixNumberOperator("*", func(a, b float64) (interface{}, error) { return a * b, nil }),
	InfixNumberOperator("/", func(a, b float64) (interface{}, error) { return a / b, nil }),
	InfixNumberOperator("%", func(a, b float64) (interface{}, error) { return math.Mod(a, b), nil }),
	InfixNumberOperator("**", func(a, b float64) (interface{}, error) { return math.Pow(a, b), nil }),

	InfixNumberOperator(">", func(a, b float64) (interface{}, error) { return a > b, nil }),
	InfixNumberOperator(">=", func(a, b float64) (interface{}, error) { return a >= b, nil }),
	InfixNumberOperator("<", func(a, b float64) (interface{}, error) { return a < b, nil }),
	InfixNumberOperator("<=", func(a, b float64) (interface{}, error) { return a <= b, nil }),

	InfixNumberOperator("==", func(a, b float64) (interface{}, error) { return a == b, nil }),
	InfixNumberOperator("!=", func(a, b float64) (interface{}, error) { return a != b, nil }),

	base,
)

var bitmask = NewLanguage(
	InfixNumberOperator("^", func(a, b float64) (interface{}, error) { return float64(int64(a) ^ int64(b)), nil }),
	InfixNumberOperator("&", func(a, b float64) (interface{}, error) { return float64(int64(a) & int64(b)), nil }),
	InfixNumberOperator("|", func(a, b float64) (interface{}, error) { return float64(int64(a) | int64(b)), nil }),
	InfixNumberOperator("<<", func(a, b float64) (interface{}, error) { return float64(int64(a) << uint64(b)), nil }),
	InfixNumberOperator(">>", func(a, b float64) (interface{}, error) { return float64(int64(a) >> uint64(b)), nil }),

	PrefixOperator("~", func(c context.Context, v interface{}) (interface{}, error) {
		i, ok := convertToFloat(v)
		if !ok {
			return nil, fmt.Errorf("unexpected %T expected number", v)
		}
		return float64(^int64(i)), nil
	}),
)

var text = NewLanguage(
	InfixTextOperator("+", func(a, b string) (interface{}, error) { return fmt.Sprintf("%v%v", a, b), nil }),

	InfixTextOperator("<", func(a, b string) (interface{}, error) { return a < b, nil }),
	InfixTextOperator("<=", func(a, b string) (interface{}, error) { return a <= b, nil }),
	InfixTextOperator(">", func(a, b string) (interface{}, error) { return a > b, nil }),
	InfixTextOperator(">=", func(a, b string) (interface{}, error) { return a >= b, nil }),

	InfixEvalOperator("=~", regEx),
	InfixEvalOperator("!~", notRegEx),
	base,
)

var propositionalLogic = NewLanguage(
	PrefixOperator("!", func(c context.Context, v interface{}) (interface{}, error) {
		b, ok := convertToBool(v)
		if !ok {
			return nil, fmt.Errorf("unexpected %T expected bool", v)
		}
		return !b, nil
	}),

	InfixShortCircuit("&&", func(a interface{}) (interface{}, bool) { return false, a == false }),
	InfixBoolOperator("&&", func(a, b bool) (interface{}, error) { return a && b, nil }),
	InfixShortCircuit("||", func(a interface{}) (interface{}, bool) { return true, a == true }),
	InfixBoolOperator("||", func(a, b bool) (interface{}, error) { return a || b, nil }),

	InfixBoolOperator("==", func(a, b bool) (interface{}, error) { return a == b, nil }),
	InfixBoolOperator("!=", func(a, b bool) (interface{}, error) { return a != b, nil }),

	base,
)

var base = NewLanguage(
	PrefixExtension(scanner.Int, parseNumber),
	PrefixExtension(scanner.Float, parseNumber),
	PrefixOperator("-", func(c context.Context, v interface{}) (interface{}, error) {
		i, ok := convertToFloat(v)
		if !ok {
			return nil, fmt.Errorf("unexpected %v(%T) expected number", v, v)
		}
		return -i, nil
	}),

	PrefixExtension(scanner.String, parseString),
	PrefixExtension(scanner.Char, parseString),
	PrefixExtension(scanner.RawString, parseString),

	Constant("true", true),
	Constant("false", false),

	InfixOperator("==", func(a, b interface{}) (interface{}, error) {
		aa, ok := a.([]interface{})
		if !ok {
			return reflect.DeepEqual(a, b), nil
		}
		for _, x := range aa {
			if reflect.DeepEqual(x, b) {
				return true, nil
			}
		}
		return false, nil
	}),
	InfixOperator("!=", func(a, b interface{}) (interface{}, error) {
		aa, ok := a.([]interface{})
		if !ok {
			return !reflect.DeepEqual(a, b), nil
		}
		for _, x := range aa {
			if !reflect.DeepEqual(x, b) {
				return true, nil
			}
		}
		return false, nil
	}),
	// InfixOperator(">", func(a, b interface{}) (interface{}, error) {
	// 	aa, ok := a.([]interface{})
	// 	if !ok {
	// 		return compareGreaterThan(a, b)
	// 	}
	// 	for _, x := range aa {
	// 		if val, err := compareGreaterThan(x, b); val == true {
	// 			return true, err
	// 		}
	// 	}
	// 	return false, nil
	// }),
	// InfixOperator(">=", func(a, b interface{}) (interface{}, error) {
	// 	aa, ok := a.([]interface{})
	// 	if !ok {
	// 		return compareGreaterEqualThan(a, b)
	// 	}
	// 	for _, x := range aa {
	// 		if val, err := compareGreaterEqualThan(x, b); val == true {
	// 			return true, err
	// 		}
	// 	}
	// 	return false, nil
	// }),
	PrefixExtension('(', parseParentheses),

	Precedence("??", 0),

	Precedence("||", 20),
	Precedence("&&", 21),

	Precedence("==", 40),
	Precedence("!=", 40),
	Precedence(">", 40),
	Precedence(">=", 40),
	Precedence("<", 40),
	Precedence("<=", 40),
	Precedence("=~", 40),
	Precedence("!~", 40),
	Precedence("in", 40),

	Precedence("^", 60),
	Precedence("&", 60),
	Precedence("|", 60),

	Precedence("<<", 90),
	Precedence(">>", 90),

	Precedence("+", 120),
	Precedence("-", 120),

	Precedence("*", 150),
	Precedence("/", 150),
	Precedence("%", 150),

	Precedence("**", 200),

	PrefixMetaPrefix(scanner.Ident, parseIdent),
)

func compareGreaterThan(a, b interface{}) (val interface{}, err error) {

	if a == nil || b == nil {

		val = errors.New("invalid operation")
		return

	}
	typeA := reflect.TypeOf(a)
	typeB := reflect.TypeOf(b)
	fmt.Println(typeA.Kind())

	if typeA.Kind() != typeB.Kind() {
		val = "invalid operation (" + typeA.Kind().String() + ") > (" + typeB.Kind().String() + ")"
		return
	}
	switch typeA.Kind() {
	case reflect.Int:
		val = a.(int) > b.(int)
		break
	case reflect.Int8:
		val = a.(int8) > b.(int8)
		break
	case reflect.Int16:
		val = a.(int16) > b.(int16)
		break
	case reflect.Int32:
		val = a.(int32) > b.(int32)
		break
	case reflect.Int64:
		val = a.(int64) > b.(int64)
		break
	case reflect.Uint:
		val = a.(uint) > b.(uint)
		break
	case reflect.Uint8:
		val = a.(uint8) > b.(uint8)
		break
	case reflect.Uint16:
		val = a.(uint16) > b.(uint16)
		break
	case reflect.Uint32:
		val = a.(uint32) > b.(uint32)
		break
	case reflect.Uint64:
		val = a.(uint64) > b.(uint64)
		break
	case reflect.Float32:
		val = a.(float32) > b.(float32)
		break
	case reflect.Float64:
		val = a.(float64) > b.(float64)
		break
	default:
		val = false
		break
	}
	return
}

func compareGreaterEqualThan(a, b interface{}) (val interface{}, err error) {

	if a == nil || b == nil {

		val = errors.New("invalid operation")
		return

	}
	typeA := reflect.TypeOf(a)
	typeB := reflect.TypeOf(b)

	if typeA.Kind() != typeB.Kind() {
		val = errors.New("invalid operation")
		return
	}
	switch typeA.Kind() {
	case reflect.Bool:
		val = a.(bool) == b.(bool)
		break
	case reflect.Int:
		val = a.(int) >= b.(int)
		break
	case reflect.Int8:
		val = a.(int8) >= b.(int8)
		break
	case reflect.Int16:
		val = a.(int16) >= b.(int16)
		break
	case reflect.Int32:
		val = a.(int32) >= b.(int32)
		break
	case reflect.Int64:
		val = a.(int64) >= b.(int64)
		break
	case reflect.Uint:
		val = a.(uint) >= b.(uint)
		break
	case reflect.Uint8:
		val = a.(uint8) >= b.(uint8)
		break
	case reflect.Uint16:
		val = a.(uint16) >= b.(uint16)
		break
	case reflect.Uint32:
		val = a.(uint32) >= b.(uint32)
		break
	case reflect.Uint64:
		val = a.(uint64) >= b.(uint64)
		break
	case reflect.Float32:
		val = a.(float32) >= b.(float32)
		break
	case reflect.Float64:
		val = a.(float64) >= b.(float64)
		break
	}
	return
}

func compareLessThan(a, b interface{}) bool {
	if a == nil || b == nil {
		return false
	}
	typeA := reflect.TypeOf(a)
	typeB := reflect.TypeOf(b)
	if typeA != typeB {
		return false
	}
	switch typeA.Kind() {
	case reflect.Int:
		return a.(int) < b.(int)
	case reflect.Int8:
		return a.(int8) < b.(int8)
	case reflect.Int16:
		return a.(int16) < b.(int16)
	case reflect.Int32:
		return a.(int32) < b.(int32)
	case reflect.Int64:
		return a.(int64) < b.(int64)
	case reflect.Uint:
		return a.(uint) < b.(uint)
	case reflect.Uint8:
		return a.(uint8) < b.(uint8)
	case reflect.Uint16:
		return a.(uint16) < b.(uint16)
	case reflect.Uint32:
		return a.(uint32) < b.(uint32)
	case reflect.Uint64:
		return a.(uint64) < b.(uint64)
	case reflect.Float32:
		return a.(float32) < b.(float32)
	case reflect.Float64:
		return a.(float64) < b.(float64)

	}
	return false
}

func compareLessEqualThan(a, b interface{}) bool {
	if a == nil || b == nil {
		return false
	}
	typeA := reflect.TypeOf(a)
	typeB := reflect.TypeOf(b)
	if typeA != typeB {
		return false
	}
	switch typeA.Kind() {
	case reflect.Int:
		return a.(int) <= b.(int)
	case reflect.Int8:
		return a.(int8) <= b.(int8)
	case reflect.Int16:
		return a.(int16) <= b.(int16)
	case reflect.Int32:
		return a.(int32) <= b.(int32)
	case reflect.Int64:
		return a.(int64) <= b.(int64)
	case reflect.Uint:
		return a.(uint) <= b.(uint)
	case reflect.Uint8:
		return a.(uint8) <= b.(uint8)
	case reflect.Uint16:
		return a.(uint16) <= b.(uint16)
	case reflect.Uint32:
		return a.(uint32) <= b.(uint32)
	case reflect.Uint64:
		return a.(uint64) <= b.(uint64)
	case reflect.Float32:
		return a.(float32) <= b.(float32)
	case reflect.Float64:
		return a.(float64) <= b.(float64)

	}
	return false
}
