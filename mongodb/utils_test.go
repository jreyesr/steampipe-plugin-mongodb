package mongodb

import (
	"context"
	"github.com/hashicorp/go-hclog"
	"github.com/jreyesr/steampipe-plugin-mongodb/mongodb/analyzer"
	"github.com/turbot/steampipe-plugin-sdk/v5/grpc/proto"
	"github.com/turbot/steampipe-plugin-sdk/v5/plugin"
	"github.com/turbot/steampipe-plugin-sdk/v5/plugin/context_key"
	"github.com/turbot/steampipe-plugin-sdk/v5/plugin/quals"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"reflect"
	"testing"
	"time"
)

func ctx() context.Context {
	return context.WithValue(context.Background(), context_key.Logger, hclog.Default())
}

func makeQual(column, op string, val any) plugin.KeyColumnQualMap {
	return plugin.KeyColumnQualMap{
		column: {Name: column, Quals: []*quals.Qual{{column, op, proto.NewQualValue(val)}}},
	}
}

var typeMap = analyzer.StructType{
	"_id": analyzer.PrimitiveObjectId,
	"field": analyzer.StructType{
		"string": analyzer.PrimitiveString,
		"ts":     analyzer.PrimitiveTimestamp,
	},
}
var columns = []*plugin.Column{
	{Name: "_id", Type: proto.ColumnType_STRING},
	{Name: "field.string", Type: proto.ColumnType_STRING},
	{Name: "field.ts", Type: proto.ColumnType_TIMESTAMP},
}

func TestStringQual(t *testing.T) {
	qual := makeQual("field.string", "=", "val")

	filter := qualsToMongoFilter(ctx(), qual, columns, typeMap)
	expected := bson.D{{"field.string", bson.M{"$eq": "val"}}}

	if !reflect.DeepEqual(filter, expected) {
		t.Errorf("Expected filter to be %v but it was %v", expected, filter)
	}
}

func TestTimestampQual(t *testing.T) {
	qual := makeQual("field.ts", "<=", time.Unix(0, 0))

	filter := qualsToMongoFilter(ctx(), qual, columns, typeMap)
	expected := bson.D{{"field.ts", bson.M{"$lte": time.Unix(0, 0).UTC()}}}

	if !reflect.DeepEqual(filter, expected) {
		t.Errorf("Expected filter to be %v but it was %v", expected, filter)
	}
}

func TestRegexQual(t *testing.T) {
	qual := makeQual("field.string", "!~*", ".*")

	filter := qualsToMongoFilter(ctx(), qual, columns, typeMap)
	expected := bson.D{{"field.string", bson.M{"$not": bson.M{"$regex": ".*", "$options": "i"}}}}

	if !reflect.DeepEqual(filter, expected) {
		t.Errorf("Expected filter to be %v but it was %v", expected, filter)
	}
}

func TestObjectIDQual(t *testing.T) {
	oid := primitive.NewObjectID()
	qual := makeQual("_id", "=", oid.Hex())

	filter := qualsToMongoFilter(ctx(), qual, columns, typeMap)
	expected := bson.D{{"_id", bson.M{"$eq": oid}}}

	if !reflect.DeepEqual(filter, expected) {
		t.Errorf("Expected filter to be %v but it was %v", expected, filter)
	}
}
