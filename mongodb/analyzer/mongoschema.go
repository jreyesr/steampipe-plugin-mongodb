package analyzer

import (
	"bytes"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// Generator holds state about a set of documents, so they can be incrementally analyzed.
// Usage: create a new Generator, then repeatedly call Update passing a MongoDB document each time. When done, call
// GetType once to retrieve the final type inferred from all the passed documents
type Generator struct {
	// StopOnFields indicates a set of fields that will not be drilled into. Use, for example, for objects with high-cardinality keys, i.e. where the key of the object is _itself_ a variable value, such as an ID
	StopOnFields  []string
	positionStack []string

	root StructType
}

// Update adds a new MongoDB document to the Generator's internal state
func (gen *Generator) Update(m bson.M) error {
	if gen.root == nil {
		gen.root = StructType{}
	}

	gen.root.Merge(gen.TypeOf(m, nil), gen)
	return nil
}

// GetType returns the final inferred type, which has been inferred from all the documents that were provided to Update
func (gen *Generator) GetType() Type {
	return gen.root
}

type Type interface {
	GoType(gen *Generator) string
	Merge(t Type, gen *Generator) Type
}

// LiteralType is for actual values, currently only NilType
type LiteralType struct {
	Literal string
}

func (l LiteralType) GoType(gen *Generator) string {
	return l.Literal
}

func (l LiteralType) Merge(t Type, gen *Generator) Type {
	if l.GoType(gen) == t.GoType(gen) {
		return l
	}
	return MixedType{l, t}
}

var NilType = LiteralType{Literal: "nil"}

type MixedType []Type

func (m MixedType) GoType(gen *Generator) string {
	return "interface{}"
}

func (m MixedType) Merge(t Type, gen *Generator) Type {
	// New type, t, is already one in the list of MixedType, so just return the original list
	for _, e := range m {
		if e.GoType(gen) == t.GoType(gen) {
			return m
		}
	}
	// Otherwise, return a new list with a new type added to the end
	return append(m, t)
}

type PrimitiveType uint

const (
	PrimitiveBool PrimitiveType = iota
	PrimitiveDouble
	PrimitiveInt32
	PrimitiveInt64
	PrimitiveDecimal
	PrimitiveString
	PrimitiveBinary
	PrimitiveObjectId
	PrimitiveRegex
	PrimitiveJS
	PrimitiveScopedCode
	PrimitiveSymbol
	PrimitiveDateTime
	PrimitiveTimestamp
	PrimitiveDBPointer
	PrimitiveMinKey
	PrimitiveMaxKey
	PrimitiveUndefined
)

func (p PrimitiveType) GoType(gen *Generator) string {
	switch p {
	case PrimitiveBool:
		return "bool"
	case PrimitiveDouble:
		return "float64"
	case PrimitiveInt32:
		return "int32"
	case PrimitiveInt64:
		return "int64"
	case PrimitiveDecimal:
		return "bson.Decimal128"
	case PrimitiveString:
		return "string"
	case PrimitiveBinary:
		return "bson.Binary"
	case PrimitiveObjectId:
		return "bson.ObjectId"
	case PrimitiveRegex:
		return "bson.Regex"
	case PrimitiveJS:
		return "bson.JavaScript"
	case PrimitiveScopedCode:
		return "bson.CodeWithScope"
	case PrimitiveSymbol:
		return "bson.Symbol"
	case PrimitiveDateTime:
		return "bson.DateTime"
	case PrimitiveTimestamp:
		return "time.Time"
	case PrimitiveDBPointer:
		return "bson.DBPointer"
	case PrimitiveMinKey:
		return "bson.MinKey"
	case PrimitiveMaxKey:
		return "bson.MaxKey"
	case PrimitiveUndefined:
		return "bson.Undefined"
	}
	panic(fmt.Sprintf("unknown primitive: %v %d", p, uint(p)))
}

func (p PrimitiveType) Merge(t Type, gen *Generator) Type {
	if p.GoType(gen) == t.GoType(gen) {
		return p
	}
	return MixedType{p, t}
}

type SliceType struct {
	Type
}

func (s SliceType) GoType(gen *Generator) string {
	return fmt.Sprintf("[]%s", s.Type.GoType(gen))
}

func (s SliceType) Merge(t Type, gen *Generator) Type {
	if s.GoType(gen) == t.GoType(gen) {
		return s
	}

	// If the target type is a slice of structs, we merge into the first struct
	// type in our own slice type.
	if targetSliceType, ok := t.(SliceType); ok {
		if targetSliceStructType, ok := targetSliceType.Type.(StructType); ok {
			// We're a slice of structs.
			if ownSliceStructType, ok := s.Type.(StructType); ok {
				s.Type = ownSliceStructType.Merge(targetSliceStructType, gen)
				return s
			}

			// We're a slice of mixed types, one of which may or may not be a struct.
			if sliceMixedType, ok := s.Type.(MixedType); ok {
				for i, v := range sliceMixedType {
					if vStructType, ok := v.(StructType); ok {
						sliceMixedType[i] = vStructType.Merge(targetSliceStructType, gen)
						return s
					}
				}
				return SliceType{Type: append(sliceMixedType, targetSliceStructType)}
			}
		}
	}
	return MixedType{s, t}
}

type StructType map[string]Type

func (s StructType) GoType(gen *Generator) string {
	var buf bytes.Buffer
	fmt.Fprintln(&buf, "struct {")
	for k, v := range s {
		if isValidFieldName(k) {
			vGoType := v.GoType(gen)

			fmt.Fprintf(
				&buf,
				"%s %s `bson:\"%s,omitempty\"`\n",
				makeFieldName(k),
				vGoType,
				k,
			)
		}
	}
	fmt.Fprint(&buf, "}")
	return buf.String()
}

func (s StructType) Merge(t Type, gen *Generator) Type {
	// merge(struct1, struct2) = {field1: merge(struct1.field1, struct2.field1), ...}
	// i.e. recursively merge the same-named fields on both structs
	if o, ok := t.(StructType); ok {
		for k, v := range o {
			if e, ok := s[k]; ok {
				s[k] = e.Merge(v, gen)
			} else {
				s[k] = v
			}
		}
		return s
	}
	// merge(struct, anything else) = Union[struct, anything else]
	return MixedType{s, t}
}

// TypeOf receives an arbitrary value v, taken from a MongoDB database, and returns the Type that
// the value maps to. The stack parameter is the current path (in the entire document) that this value is located at,
// for example, if the original doc is {a: {b: 1}}, it'd be TypeOf({b: 1}, {"a"}), or TypeOf(1, {"a", "b"})
func (gen *Generator) TypeOf(v interface{}, stack []string) Type {
	if stack == nil {
		stack = []string{}
	}

	switch i := v.(type) {
	default:
		panic(fmt.Sprintf("cannot determine type for %v with go type %T", v, v))
	case int32:
		return PrimitiveInt32
	case int64:
		return PrimitiveInt64
	case float64:
		return PrimitiveDouble
	case string:
		return PrimitiveString
	case bool:
		return PrimitiveBool
	case primitive.M:
		return NewStructType(i, gen, stack)
	case primitive.D:
		return NewOrderedStructType(i, gen, stack)
	case primitive.A:
		return NewArrayType(i, gen, stack)
	case primitive.ObjectID:
		return PrimitiveObjectId
	case primitive.DateTime:
		return PrimitiveDateTime
	case primitive.Binary:
		return PrimitiveBinary
	case primitive.Regex:
		return PrimitiveRegex
	case primitive.JavaScript:
		return PrimitiveJS
	case primitive.CodeWithScope:
		return PrimitiveScopedCode
	case primitive.Timestamp:
		return PrimitiveTimestamp
	case primitive.Decimal128:
		return PrimitiveDecimal
	case primitive.MinKey:
		return PrimitiveMinKey
	case primitive.MaxKey:
		return PrimitiveMaxKey
	case primitive.Undefined:
		return PrimitiveUndefined
	case nil:
		return NilType
	case primitive.DBPointer:
		return PrimitiveDBPointer
	case primitive.Symbol:
		return PrimitiveSymbol
	}
}

func NewStructType(m bson.M, gen *Generator, stack []string) Type {
	s := StructType{}
	for k, v := range m {
		currentFieldName := strings.Join(append(stack, k), ".")
		// Check if this subfield is one of the ignored ones.
		if slices.Contains(gen.StopOnFields, currentFieldName) {
			// If so, just report its type as a generic Struct[?], as if it had no children fields to begin with
			s[k] = StructType{}
		} else {
			// Otherwise, recurse into the subfield and analyze _its_ type
			t := gen.TypeOf(v, append(stack, k))
			s[k] = t
		}
	}
	return s
}

func NewOrderedStructType(d bson.D, gen *Generator, stack []string) Type {
	s := StructType{}
	for _, f := range d {
		k, v := f.Key, f.Value
		t := gen.TypeOf(v, append(stack, k))
		if t == NilType {
			continue
		}
		s[k] = t
	}
	return s
}

func NewArrayType(d bson.A, gen *Generator, stack []string) Type {
	if len(d) == 0 {
		return SliceType{Type: MixedType{}}
	}
	var s Type
	for _, v := range d {
		vt := gen.TypeOf(v, stack) // Arrays don't push a new stack context
		if s == nil {
			s = SliceType{Type: vt}
		} else {
			s.Merge(SliceType{Type: vt}, gen)
		}
	}
	if s == nil {
		return SliceType{Type: MixedType{}}
	}
	return s
}

func isValidFieldName(n string) bool {
	if n == "" {
		return false
	}
	if strings.IndexAny(n, "!*") == -1 {
		return true
	}
	return false
}

var (
	dashUnderscoreReplacer = strings.NewReplacer("-", " ", "_", " ")
	capsRe                 = regexp.MustCompile(`([A-Z])`)
	spaceRe                = regexp.MustCompile(`(\w+)`)
	forcedUpperCase        = map[string]bool{"id": true, "url": true, "api": true}
)

func split(str string) []string {
	str = dashUnderscoreReplacer.Replace(str) // my_field_longName -> my field longName
	str = capsRe.ReplaceAllString(str, " $1") // my field longName -> my field long name
	return spaceRe.FindAllString(str, -1)     // my field long name -> ["my", "field", "long", "name"]
}

func makeFieldName(s string) string {
	titleCaser := cases.Title(language.English)
	parts := split(s)
	for i, part := range parts {
		if forcedUpperCase[strings.ToLower(part)] {
			parts[i] = strings.ToUpper(part)
		} else {
			parts[i] = titleCaser.String(part)
		}
	}
	camel := strings.Join(parts, "")
	runes := []rune(camel)
	for i, c := range runes {
		ok := unicode.IsLetter(c) || unicode.IsDigit(c)
		if i == 0 {
			ok = unicode.IsLetter(c)
		}
		if !ok {
			runes[i] = '_'
		}
	}
	return string(runes)
}
