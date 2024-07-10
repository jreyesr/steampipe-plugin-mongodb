package mongodb

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/google/uuid"
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
	samplingPipeline := bson.D{
		{"$sample", bson.M{"size": 4}},
	}
	cursor, err := collection.Aggregate(ctx, mongo.Pipeline{samplingPipeline})
	if err != nil {
		return nil, err
	}

	seenFields := make(map[string][]proto.ColumnType)
	var results []bson.D
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	for _, result := range results {
		for _, v := range result { // for each field in the sampled document
			t := getSteampipeTypeForMongoValue(ctx, v.Value)

			if !slices.Contains(seenFields[v.Key], t) {
				seenFields[v.Key] = append(seenFields[v.Key], t)
			}
		}
	}

	finalTypes := make(map[string]proto.ColumnType, len(seenFields))
	for f, possibleTypes := range seenFields {
		if len(possibleTypes) == 1 {
			finalTypes[f] = possibleTypes[0]
		} else {
			finalTypes[f] = proto.ColumnType_JSON
		}
	}

	return finalTypes, nil
}

func getSteampipeTypeForMongoValue(ctx context.Context, val any) proto.ColumnType {
	switch val.(type) {
	// https://pkg.go.dev/go.mongodb.org/mongo-driver@v1.16.0/bson#hdr-Native_Go_Types
	case int32, int64:
		return proto.ColumnType_INT
	case float64, primitive.Decimal128:
		return proto.ColumnType_DOUBLE
	case string, primitive.ObjectID, primitive.Binary, primitive.Regex, primitive.JavaScript, primitive.CodeWithScope, primitive.Symbol:
		return proto.ColumnType_STRING
	case bool:
		return proto.ColumnType_BOOL
	case nil, primitive.M, primitive.D, primitive.A, primitive.Undefined, primitive.DBPointer:
		return proto.ColumnType_JSON
	case primitive.DateTime, primitive.Timestamp:
		return proto.ColumnType_TIMESTAMP
	case primitive.MinKey, primitive.MaxKey:
		return proto.ColumnType_UNKNOWN
	default:
		plugin.Logger(ctx).Error("mongodb.getSteampipeTypeForMongoValue", "msg", "unknown type", "val", val)
		return proto.ColumnType_UNKNOWN
	}
}

// FromSingleField is similar to [transform.FromField], except that it doesn't support
// checking on multiple fields, just one, and it also doesn't check for nilness using [reflect.Value.IsNil], because
// that function call breaks when using [primitive.ObjectID], which is an alias to [12]byte
func FromSingleField(_ context.Context, d *transform.TransformData) (any, error) {
	fieldName := d.Param.(string)
	entireItem := d.HydrateItem

	fieldValue, ok := helpers.GetNestedFieldValueFromInterface(entireItem, fieldName)
	if !ok {
		return nil, fmt.Errorf("unable to extract field %s from item %v", fieldName, entireItem)
	}
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
			case proto.ColumnType_INT:
				filterValue = qual.Value.GetInt64Value()
			case proto.ColumnType_DOUBLE:
				filterValue = qual.Value.GetDoubleValue()
			case proto.ColumnType_BOOL:
				filterValue = qual.Value.GetBoolValue()
			}

			var operatorMap = map[string]string{
				quals.QualOperatorEqual:          "$eq",
				quals.QualOperatorNotEqual:       "$ne",
				quals.QualOperatorGreater:        "$gt",
				quals.QualOperatorLess:           "$lt",
				quals.QualOperatorGreaterOrEqual: "$gte",
				quals.QualOperatorLessOrEqual:    "$lte",
				quals.QualOperatorIsNull:         "$eq",
				quals.QualOperatorIsNotNull:      "$ne",
				// (not) (i)like (x4)
				// (not) (i)regex (x4)

				// quals.QualOperatorJsonbContainsLeftRight,
				// quals.QualOperatorJsonbContainsRightLeft,
				// quals.QualOperatorJsonbExistsOne,
				// quals.QualOperatorJsonbExistsAny,
				// quals.QualOperatorJsonbExistsAll,
				// quals.QualOperatorJsonbPathExists,
				// quals.QualOperatorJsonbPathPredicate,
			}

			// For example, {"age": {"$gt": 1.2}}
			filter = append(filter, bson.E{Key: qual.Column, Value: bson.M{operatorMap[qual.Operator]: filterValue}})
		}
	}
	return filter
}
