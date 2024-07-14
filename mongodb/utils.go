package mongodb

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/google/uuid"
	"github.com/jreyesr/steampipe-plugin-mongodb/mongodb/analyzer"
	"github.com/turbot/go-kit/helpers"
	"github.com/turbot/steampipe-plugin-sdk/v5/grpc/proto"
	"github.com/turbot/steampipe-plugin-sdk/v5/plugin"
	"github.com/turbot/steampipe-plugin-sdk/v5/plugin/quals"
	"github.com/turbot/steampipe-plugin-sdk/v5/plugin/transform"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"slices"
	"strconv"
	"time"
)

func connect(ctx context.Context, connectionString string) (*mongo.Client, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(connectionString))
	return client, err
}

func getCollectionsOnDatabase(ctx context.Context, connectionString string, dbName string) ([]string, error) {
	client, err := connect(ctx, connectionString)
	if err != nil {
		return nil, err
	}

	collNames, err := client.Database(dbName).ListCollectionNames(ctx, bson.D{})
	return collNames, err
}

func getFieldTypesForCollection(ctx context.Context, collection *mongo.Collection) (map[string]proto.ColumnType, error) {
	// grab some random docs from the collection
	// TODO: The sample val should be configurable by end user
	samplingPipeline := bson.D{
		{"$sample", bson.M{"size": 4}},
	}
	cursor, err := collection.Aggregate(ctx, mongo.Pipeline{samplingPipeline})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	g := analyzer.Generator{}

	for cursor.Next(ctx) {
		var sampleDoc bson.M
		if err := cursor.Decode(&sampleDoc); err != nil {
			return nil, err
		}
		// Feed this new document into the Generator, so it updates its type map
		g.Update(sampleDoc)
	}
	// After feeding all the sample docs into the Generator, read out the final type map
	typeMap := g.GetType().(analyzer.StructType)
	// typeMap is a specification inferred from ALL the observed documents (those that were passed to [analyzer.Generator.Update])
	// typeMap may look like this:
	// StructType {
	//   "_id": PrimitiveObjectID,
	//   "name": StructType {
	//     "first": PrimitiveString,
	//     "last": PrimitiveString,
	//   },
	//   "num_purchases": PrimitiveInt64,
	//   "active_features": SliceType{PrimitiveString},
	// }

	finalTypes := map[string]proto.ColumnType{}
	for fieldName, fieldType := range typeMap {
		// This runs over each top-level field in the inferred schema
		// Most fields will return just a single item, but nested structs/documents may return several entries, because
		// nested documents will recurse.
		// For example, if the type of "name" is StructType {"first": PrimitiveString, "last": PrimitiveString}
		// (based on observing documents that look, say, like {name: {first: "John", last: "Doe"}})
		// then thisFieldColumns will be {"name.first": proto.ColumnType_STRING, "name.last": proto.ColumnType_STRING}
		thisFieldColumns := mongoFieldToSteampipeCol(ctx, fieldName, fieldType)
		for k, v := range thisFieldColumns {
			finalTypes[k] = v
		}
	}

	return finalTypes, nil
}

// getSteampipeTypeForMongoType translates Mongo types, as used in the [analyzer] package, and converts them to Steampipe-specific
// types from [proto], such as [proto.ColumnType_JSON]. Some rules:
//   - Literal types (currently only nil) become JSONB
//   - String(ish) fields, namely Binary, ObjectID, Regex, Javascript (with&without scope) and Symbol, become TEXT
//   - DBPointer becomes JSONB
//   - Minkey, Maxkey and Undefined become UNKNOWN columns, which are later dropped
//   - The rest of primitive types are translated directly (strings, numbers, booleans)
//   - Array fields become JSONB
//   - Nested documents/subdocuments that have no internal fields become JSONB
//   - Nested documents that _do_ have fields are "exploded" into a column for each of the child fields, where the name is the parent's name, then a period (.), and then the child's name. This case is capable of recursion
func getSteampipeTypeForMongoType(ctx context.Context, mongoType analyzer.Type) proto.ColumnType {
	switch mongoType.(type) {
	default:
		plugin.Logger(ctx).Error("mongodb.getSteampipeTypeForMongoType", "msg", "unknown type", "mongoType", mongoType)
		return proto.ColumnType_UNKNOWN
	case analyzer.LiteralType:
		switch mongoType {
		default:
			plugin.Logger(ctx).Error("mongodb.getSteampipeTypeForMongoType", "msg", "unknown literal type", "mongoType", mongoType)
			return proto.ColumnType_UNKNOWN
		case analyzer.NilType:
			return proto.ColumnType_JSON
		}
	case analyzer.MixedType:
		return proto.ColumnType_JSON
	case analyzer.PrimitiveType:
		switch mongoType {
		default:
			plugin.Logger(ctx).Error("mongodb.getSteampipeTypeForMongoType", "msg", "unknown primitive type", "mongoType", mongoType)
			return proto.ColumnType_UNKNOWN
		case analyzer.PrimitiveBool:
			return proto.ColumnType_BOOL
		case analyzer.PrimitiveDouble:
			return proto.ColumnType_DOUBLE
		case analyzer.PrimitiveInt32:
			return proto.ColumnType_INT
		case analyzer.PrimitiveInt64:
			return proto.ColumnType_INT
		case analyzer.PrimitiveDecimal:
			return proto.ColumnType_DOUBLE
		case analyzer.PrimitiveString:
			return proto.ColumnType_STRING
		case analyzer.PrimitiveBinary:
			return proto.ColumnType_STRING
		case analyzer.PrimitiveObjectId:
			return proto.ColumnType_STRING
		case analyzer.PrimitiveRegex:
			return proto.ColumnType_STRING
		case analyzer.PrimitiveJS:
			return proto.ColumnType_STRING
		case analyzer.PrimitiveScopedCode:
			return proto.ColumnType_STRING
		case analyzer.PrimitiveSymbol:
			return proto.ColumnType_STRING
		case analyzer.PrimitiveDateTime:
			return proto.ColumnType_TIMESTAMP
		case analyzer.PrimitiveTimestamp:
			return proto.ColumnType_TIMESTAMP
		case analyzer.PrimitiveDBPointer:
			return proto.ColumnType_JSON
		case analyzer.PrimitiveMinKey:
			return proto.ColumnType_UNKNOWN
		case analyzer.PrimitiveMaxKey:
			return proto.ColumnType_UNKNOWN
		case analyzer.PrimitiveUndefined:
			return proto.ColumnType_UNKNOWN
		}
	case analyzer.SliceType:
		// Don't even look into the
		return proto.ColumnType_JSON
	case analyzer.StructType:
		return proto.ColumnType_JSON
	}
}

func mongoFieldToSteampipeCol(ctx context.Context, fieldName string, fieldType analyzer.Type) map[string]proto.ColumnType {
	// Only recurse IF this field is an object AND it has at least one child field
	// For example: fieldName=contactInfo, fieldType=StructType{name: PrimitiveString, email: PrimitiveString}
	if childTypeMap, ok := fieldType.(analyzer.StructType); ok && len(childTypeMap) > 0 {
		allColumns := make(map[string]proto.ColumnType)

		for childFieldName, typeOfChildField := range childTypeMap {
			// Give each child field an opportunity to present its own fields
			childFieldFullName := fmt.Sprintf("%s.%s", fieldName, childFieldName)
			childFields := mongoFieldToSteampipeCol(ctx, childFieldFullName, typeOfChildField)
			for k, v := range childFields {
				allColumns[k] = v
			}
		}
		return allColumns
	}

	// No other cases recurse, so we just return that single field as a column
	// This includes: literal null, mixed-type fields, primitives (e.g. strings, ObjectIDs, integers, regex), arrays, and empty objects (those that don't have child info)
	return map[string]proto.ColumnType{fieldName: getSteampipeTypeForMongoType(ctx, fieldType)}
}

// FromSingleField is similar to [transform.FromField], except that it doesn't support
// checking on multiple fields, just one, and it also doesn't check for nilness using [reflect.Value.IsNil], because
// that function call breaks when using [primitive.ObjectID], which is an alias to [12]byte
func FromSingleField(_ context.Context, d *transform.TransformData) (any, error) {
	fieldName := d.Param.(string)
	entireItem := d.HydrateItem

	fieldValue, _ := helpers.GetNestedFieldValueFromInterface(entireItem, fieldName)
	return fieldValue, nil
}

// mongoTransformFunction receives a raw field, taken from the MongoDB database, and converts it
// into a value that Steampipe can convert back into a Postgres value.
//
// For example, simple values (e.g. strings, ints or bools) are kept as-is, while ObjectIDs (which come in
// as [12]byte) are converted into their hex representation, [primitive.DateTime] is converted to Go's [time.Time],
// JS code (with or without scope) are converted into a string representation of their source code, and so on
func mongoTransformFunction(ctx context.Context, d *transform.TransformData) (any, error) {
	val := d.Value

	// Canonical list is here: https://pkg.go.dev/go.mongodb.org/mongo-driver@v1.16.0/bson#hdr-Native_Go_Types
	// MinKey and MaxKey are ignored here, because they return [proto.ColumnType_UNKNOWN] on [getSteampipeTypeForMongoValue] anyway
	switch converted := val.(type) {
	case int32, int64, float64, string, bool, nil:
		return val, nil // these are primitive values and can be returned as they are
	case primitive.M, primitive.D, primitive.A:
		return val, nil // These are wrappers over map[string]any
	case primitive.ObjectID:
		return converted.Hex(), nil
	case primitive.DateTime:
		return converted.Time(), nil
	case primitive.Binary:
		switch converted.Subtype {
		case bson.TypeBinaryUUID, bson.TypeBinaryUUIDOld:
			// treat UUIDs especially
			uu, err := uuid.FromBytes(converted.Data)
			if err != nil {
				return nil, err
			}
			return uu.String(), nil
		case bson.TypeBinaryMD5:
			// present MD5 hashes as hex strings
			return hex.EncodeToString(converted.Data), nil
		default:
			return string(converted.Data), nil // We'll send binary fields as strings. TODO: Consider using Base64 or something
		}
	case primitive.Regex:
		return converted.Pattern, nil // we lose the regex flags here, if there were any
	case primitive.JavaScript:
		return string(converted), nil
	case primitive.CodeWithScope:
		return string(converted.Code), nil // we lose the code scope here
	case primitive.Timestamp:
		return time.Unix(int64(converted.T), 0), nil
	case primitive.Decimal128:
		return strconv.ParseFloat(converted.String(), 64) // possible downcasting problems, notice that ParseFloat already returns (val, err) tuple
	case primitive.Undefined:
		return nil, nil // we arbitrarily decide that Mongo's Undefined will map to null
	case primitive.DBPointer:
		// not-that-portable representation of DBPointers, they're deprecated anyway
		return map[string]string{"db": converted.DB, "pointer": converted.Pointer.Hex()}, nil
	case primitive.Symbol:
		return string(converted), nil
	default:
		plugin.Logger(ctx).Error("mongodb.getSteampipeTypeForMongoValue", "msg", "unknown type", "val", val)
		return nil, fmt.Errorf("received unknown value %v with type %T", val, val)
	}
}

func qualsForColumnOfType(colName string, t proto.ColumnType) *plugin.KeyColumn {
	return &plugin.KeyColumn{
		Name:      colName,
		Operators: plugin.GetValidOperators(), // Accept ALL THE THINGS!
		Require:   plugin.Optional,
	}
}

func qualsToMongoFilter(ctx context.Context, inputQuals plugin.KeyColumnQualMap, columns []*plugin.Column) bson.D {
	filter := bson.D{}
	for _, filteredColumn := range inputQuals {
		for _, qual := range filteredColumn.Quals {
			colName := qual.Column
			plugin.Logger(ctx).Info("qualsToMongoFilter", qual)
			colIndex := slices.IndexFunc(columns, func(c *plugin.Column) bool { return c.Name == colName })
			col := columns[colIndex]

			var filterValue any
			switch col.Type {
			case proto.ColumnType_STRING:
				filterValue = qual.Value.GetStringValue()
			case proto.ColumnType_JSON: // Supported JSONB operators (e.g. ExistsOne) still receive strings on RHS
				filterValue = qual.Value.GetStringValue()
			case proto.ColumnType_INT:
				filterValue = qual.Value.GetInt64Value()
			case proto.ColumnType_DOUBLE:
				filterValue = qual.Value.GetDoubleValue()
			case proto.ColumnType_BOOL:
				filterValue = qual.Value.GetBoolValue()
			}

			// Not implemented, because they don't have a clean mapping to Mongo operations:
			// Combinations of (not) (i)like (x4)
			// quals.QualOperatorJsonbContainsLeftRight,
			// quals.QualOperatorJsonbContainsRightLeft,
			// quals.QualOperatorJsonbPathExists,
			// quals.QualOperatorJsonbPathPredicate,
			var filterOp bson.M
			switch qual.Operator {
			case quals.QualOperatorEqual:
				filterOp = bson.M{"$eq": filterValue}
			case quals.QualOperatorNotEqual:
				filterOp = bson.M{"$ne": filterValue}
			case quals.QualOperatorGreater:
				filterOp = bson.M{"$gt": filterValue}
			case quals.QualOperatorLess:
				filterOp = bson.M{"$lt": filterValue}
			case quals.QualOperatorGreaterOrEqual:
				filterOp = bson.M{"$gte": filterValue}
			case quals.QualOperatorLessOrEqual:
				filterOp = bson.M{"$lte": filterValue}
			case quals.QualOperatorIsNull:
				filterOp = bson.M{"$eq": nil}
			case quals.QualOperatorIsNotNull:
				filterOp = bson.M{"$ne": nil}
			case quals.QualOperatorRegex:
				filterOp = bson.M{"$regex": filterValue}
			case quals.QualOperatorNotRegex:
				filterOp = bson.M{"$not": bson.M{"$regex": filterValue}}
			case quals.QualOperatorIRegex:
				filterOp = bson.M{"$regex": filterValue, "$options": "i"}
			case quals.QualOperatorNotIRegex:
				filterOp = bson.M{"$not": bson.M{"$regex": filterValue, "$options": "i"}}
			case quals.QualOperatorJsonbExistsOne: // '["a", "b"]'::jsonb ? 'b' → t
				filterOp = bson.M{"$eq": filterValue} // {$eq: 'b'}
			case quals.QualOperatorJsonbExistsAny: // '["a", "b", "c"]'::jsonb ?| array['b', 'd'] → t
				filterOp = bson.M{"$in": filterValue} // {$in: ['b', 'd']}
			case quals.QualOperatorJsonbExistsAll: // '["a", "b", "c"]'::jsonb ?& array['a', 'b'] → t
				filterOp = bson.M{"$all": filterValue} // {$all: ['a', 'b']}
			}

			// For example, {"age": {"$gt": 1.2}}
			filter = append(filter, bson.E{Key: qual.Column, Value: filterOp})
		}
	}
	return filter
}
