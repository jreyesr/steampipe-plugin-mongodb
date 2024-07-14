package analyzer

import (
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"reflect"
	"testing"
)

func TestSimpleDocuments(t *testing.T) {
	cases := []struct {
		Val          any
		ExpectedType Type
	}{
		{int64(1), PrimitiveInt64},
		{"", PrimitiveString},
		{primitive.ObjectID{}, PrimitiveObjectId},
		{primitive.CodeWithScope{Code: "return 1", Scope: nil}, PrimitiveScopedCode},
		{nil, NilType},
		{true, PrimitiveBool},
	}

	g := Generator{}
	for _, tc := range cases {
		tc := tc // capture range variable
		t.Run(fmt.Sprintf("%#v must be of type %v", tc.Val, tc.ExpectedType), func(t *testing.T) {
			t.Parallel()
			inferredType := g.TypeOf(tc.Val, nil)
			if inferredType != tc.ExpectedType {
				t.Errorf("got %v (%T), want %v (%T)", inferredType, inferredType, tc.ExpectedType, tc.ExpectedType)
			}
		})
	}
}

func TestArray(t *testing.T) {
	g := Generator{}

	inferredType := g.TypeOf(bson.A{int64(1), int64(2), int64(3)}, nil)
	expectedType := SliceType{PrimitiveInt64}

	if inferredType != expectedType {
		t.Errorf("got %v, want %v", inferredType, expectedType)
	}
}

func TestObject(t *testing.T) {
	g := Generator{}

	inferredType := g.TypeOf(bson.M{"a": int64(1), "b": bson.A{"a", "b"}}, nil)
	expectedType := StructType{"a": PrimitiveInt64, "b": SliceType{PrimitiveString}}

	// need reflect.DeepEqual because maps are NOT comparable via !=
	if !reflect.DeepEqual(inferredType, expectedType) {
		t.Errorf("got %v, want %v", inferredType, expectedType)
	}
}

// TestObjectWithVarKeys checks for the case where an object's keys are valuable information, such as when using
// Mongo documents as a lookup table, e.g. {subscriptions: {1: true, 23: false, 45: true}} where 1, 23 and 45 are
// channel IDs (subscriptions to channels 1 and 45 are active, and subscription to channel 23 is disabled)
func TestObjectWithVarKeys(t *testing.T) {
	g := Generator{StopOnFields: []string{"subscriptions", "nested.subscriptions"}}
	obj := bson.M{
		"subscriptions": bson.M{"1": true, "23": false, "45": true},
		"stats":         bson.M{"views": int32(100), "subscriptions": bson.M{"active": int32(2), "inactive": int32(1)}},
		"nested":        bson.M{"subscriptions": bson.M{"67": true, "89": false}, "another": ""},
	}
	expectedType := StructType{
		"subscriptions": StructType{}, // this one should be ignored
		"stats": StructType{
			"views": PrimitiveInt32,
			// NOTE: We *do* want this "subscriptions" field to be analyzed, even if it has the same name
			"subscriptions": StructType{"active": PrimitiveInt32, "inactive": PrimitiveInt32},
		},
		"nested": StructType{
			"another":       PrimitiveString,
			"subscriptions": StructType{}, // this one should be ignored too
		},
	}

	inferredType := g.TypeOf(obj, nil)

	if !reflect.DeepEqual(inferredType, expectedType) {
		t.Errorf("got %v, want %v", inferredType, expectedType)
	}
}

func TestGenerator(t *testing.T) {
	objects := []string{
		"{\"_id\":{\"$oid\":\"5ca4bbcea2dd94ee58162a6d\"},\"username\":\"gregoryharrison\",\"name\":\"Natalie Ford\",\"address\":\"17677 Mark Crest\\nWalterberg, IA 39017\",\"birthdate\":{\"$date\":{\"$numberLong\":\"842634867000\"}},\"email\":\"amyholland@yahoo.com\",\"accounts\":[904260,565468],\"tier_and_details\":{\"69f8b6a3c39c42edb540499ee2651b75\":{\"tier\":\"Bronze\",\"benefits\":[ \"dedicated accountrepresentative\",\"airline lounge access\"],\"active\":true,\"id\":\"69f8b6a3c39c42edb540499ee2651b75\"},\"c85df12c2e394afb82725b16e1cc6789\":{\"tier\":\"Bronze\",\"benefits\":[\"airline lounge access\"],\"active\":true,\"id\":\"c85df12c2e394afb82725b16e1cc6789\"},\"07d516cfd7fc4ec6acf175bb78cb98a2\":{\"tier\":\"Gold\",\"benefits\":[\"dedicated account representative\"],\"active\":true,\"id\":\"07d516cfd7fc4ec6acf175bb78cb98a2\"}}}",
		// Differences:
		// * username is an array, not a string
		// * birthdate is a ISO8601 string rather than a native Mongo DateTime
		// * accounts is a single number, not an array
		// * tier_and_details is empty, but it's still an object
		"{\"_id\":{\"$oid\":\"5ca4bbcea2dd94ee58162a68\"},\"username\":[\"fmiller\"],\"name\":\"Elizabeth Ray\",\"address\":\"9286 Bethany Glens\\nVasqueztown, CO 22939\",\"birthdate\":\"1977-03-02T02:20:31.000+00:00\",\"email\":\"arroyocolton@gmail.com\",\"accounts\":904260,\"tier_and_details\":{}}",
	}
	expectedType := StructType{
		"_id":              PrimitiveObjectId,
		"username":         MixedType([]Type{PrimitiveString, SliceType{PrimitiveString}}),
		"name":             PrimitiveString,
		"address":          PrimitiveString,
		"birthdate":        MixedType([]Type{PrimitiveDateTime, PrimitiveString}),
		"email":            PrimitiveString,
		"accounts":         MixedType([]Type{SliceType{PrimitiveInt32}, PrimitiveInt32}), // Int32 because account #s are small enough
		"tier_and_details": StructType{},
	}
	g := Generator{
		StopOnFields: []string{"tier_and_details"},
	}

	for _, bsonString := range objects {
		var bsonObj bson.M
		err := bson.UnmarshalExtJSON([]byte(bsonString), false, &bsonObj)
		if err != nil {
			panic(err)
		}
		g.Update(bsonObj)
	}
	inferredType := g.GetType()

	if !reflect.DeepEqual(inferredType, expectedType) {
		t.Errorf("got %v, want %v", inferredType, expectedType)
	}
}

func TestArrayOfObjects(t *testing.T) {
	g := Generator{}
	obj := bson.A{
		bson.M{"name": "alice", "active": true},
		bson.M{"name": "bob", "active": false},
		bson.M{"name": "charlie"}, // NOTE missing active!
	}

	inferredType := g.TypeOf(obj, nil)
	expectedType := SliceType{StructType{"name": PrimitiveString, "active": PrimitiveBool}}

	if !reflect.DeepEqual(inferredType, expectedType) {
		t.Errorf("got %v, want %v", inferredType, expectedType)
	}
}

func TestEmptyArray(t *testing.T) {
	g := Generator{}

	inferredType := g.TypeOf(bson.A{}, nil)
	// Array[Union[]] is the marker for "an array, but we don't know its child types"
	expectedType := SliceType{MixedType{}}

	if !reflect.DeepEqual(inferredType, expectedType) {
		t.Errorf("got %v, want %v", inferredType, expectedType)
	}
}

func TestArrayOfNils(t *testing.T) {
	g := Generator{}

	inferredType := g.TypeOf(bson.A{nil, nil}, nil)
	// Array[Union[]] is the marker for "an array, but we don't know its child types"
	expectedType := SliceType{NilType}

	if !reflect.DeepEqual(inferredType, expectedType) {
		t.Errorf("got %v, want %v", inferredType, expectedType)
	}
}

func TestBSONKitchenSink(t *testing.T) {
	g := Generator{}
	// This is from the BSON Extended JSON spec, https://github.com/mongodb/specifications/blob/master/source/extended-json.md#canonical-extended-json-example
	bsonString := "{\"_id\":{\"$oid\":\"57e193d7a9cc81b4027498b5\"},\"String\":\"string\",\"Int32\":{\"$numberInt\":\"42\"},\"Int64\":{\"$numberLong\":\"42\"},\"Double\":{\"$numberDouble\":\"42.42\"},\"Decimal\":{\"$numberDecimal\":\"1234.5\"},\"Binary\":{\"$binary\":{\"base64\":\"yO2rw/c4TKO2jauSqRR4ow==\",\"subType\":\"04\"}},\"BinaryUserDefined\":{\"$binary\":{\"base64\":\"MTIz\",\"subType\":\"80\"}},\"Code\":{\"$code\":\"function() {}\"},\"CodeWithScope\":{\"$code\":\"function() {}\",\"$scope\":{}},\"Subdocument\":{\"foo\":\"bar\"},\"Array\":[{\"$numberInt\":\"1\"},{\"$numberInt\":\"2\"},{\"$numberInt\":\"3\"},{\"$numberInt\":\"4\"},{\"$numberInt\":\"5\"}],\"Timestamp\":{\"$timestamp\":{\"t\":42,\"i\":1}},\"RegularExpression\":{\"$regularExpression\":{\"pattern\":\"foo*\",\"options\":\"ix\"}},\"DatetimeEpoch\":{\"$date\":{\"$numberLong\":\"0\"}},\"DatetimePositive\":{\"$date\":{\"$numberLong\":\"253402300799999\"}},\"DatetimeNegative\":{\"$date\":{\"$numberLong\":\"-62135596800000\"}},\"True\":true,\"False\":false,\"DBRef\":{\"$ref\":\"collection\",\"$id\":{\"$oid\":\"57e193d7a9cc81b4027498b1\"},\"$db\":\"database\"},\"DBRefNoDB\":{\"$ref\":\"collection\",\"$id\":{\"$oid\":\"57fd71e96e32ab4225b723fb\"}},\"Minkey\":{\"$minKey\":1},\"Maxkey\":{\"$maxKey\":1},\"Null\":null}"
	expectedType := StructType{
		"_id":               PrimitiveObjectId,
		"String":            PrimitiveString,
		"Int32":             PrimitiveInt32,
		"Int64":             PrimitiveInt64,
		"Double":            PrimitiveDouble,
		"Decimal":           PrimitiveDecimal,
		"Binary":            PrimitiveBinary,
		"BinaryUserDefined": PrimitiveBinary,
		"Code":              PrimitiveJS,
		"CodeWithScope":     PrimitiveScopedCode,
		"Subdocument":       StructType{"foo": PrimitiveString},
		"Array":             SliceType{PrimitiveInt32},
		"Timestamp":         PrimitiveTimestamp,
		"RegularExpression": PrimitiveRegex,
		"DatetimeEpoch":     PrimitiveDateTime,
		"DatetimePositive":  PrimitiveDateTime,
		"DatetimeNegative":  PrimitiveDateTime,
		"True":              PrimitiveBool,
		"False":             PrimitiveBool,
		"DBRef":             StructType{"$ref": PrimitiveString, "$id": PrimitiveObjectId, "$db": PrimitiveString},
		"DBRefNoDB":         StructType{"$ref": PrimitiveString, "$id": PrimitiveObjectId},
		"Minkey":            PrimitiveMinKey,
		"Maxkey":            PrimitiveMaxKey,
		"Null":              NilType,
	}

	var bsonObj bson.M
	err := bson.UnmarshalExtJSON([]byte(bsonString), false, &bsonObj)
	if err != nil {
		panic(err)
	}
	g.Update(bsonObj)
	inferredType := g.GetType()

	if !reflect.DeepEqual(inferredType, expectedType) {
		t.Errorf("got %v, want %v", inferredType, expectedType)
	}
}
