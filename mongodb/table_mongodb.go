package mongodb

import (
	"context"
	"fmt"
	"github.com/turbot/steampipe-plugin-sdk/v5/grpc/proto"
	"github.com/turbot/steampipe-plugin-sdk/v5/plugin"
	"github.com/turbot/steampipe-plugin-sdk/v5/plugin/transform"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func tableMongoDB(ctx context.Context, connection *plugin.Connection) (*plugin.Table, error) {
	collName := ctx.Value(keyCollection).(string)
	cfg := GetConfig(connection)
	dbName := cfg.Database

	client, err := connect(ctx, *cfg.ConnectionString)
	if err != nil {
		return nil, err
	}
	coll := client.Database(dbName).Collection(collName)

	colTypes, err := getFieldTypesForCollection(ctx, coll, cfg.GetSampleSize(), cfg.GetFieldsToIgnore(collName))
	if err != nil {
		return nil, err
	}

	cols := []*plugin.Column{}
	quals := make([]*plugin.KeyColumn, 0, len(cols))
	for colName, colType := range colTypes {
		if colType == proto.ColumnType_UNKNOWN {
			plugin.Logger(ctx).Warn("Column would be unknown, ignoring instead", "column", colName)
			continue // these columns can't be presented to Steampipe
		}

		cols = append(cols, &plugin.Column{
			Name:        colName,
			Type:        colType,
			Transform:   transform.FromP(FromSingleField, colName).Transform(mongoTransformFunction),
			Description: fmt.Sprintf("Field %s", colName),
		})
		quals = append(quals, qualsForColumnOfType(colName, colType))
	}

	return &plugin.Table{
		Name:        collName,
		Description: fmt.Sprintf("Collection %s on database %s", coll.Name(), coll.Database().Name()),
		List: &plugin.ListConfig{
			Hydrate:    listMongoDBWithName(collName),
			KeyColumns: quals,
		},
		Columns: cols,
	}, nil
}

//go:noinline
func listMongoDBWithName(collName string) func(ctx context.Context, d *plugin.QueryData, h *plugin.HydrateData) (interface{}, error) {
	var _ = 1
	return func(ctx context.Context, d *plugin.QueryData, _ *plugin.HydrateData) (interface{}, error) {
		quals := d.Quals
		plugin.Logger(ctx).Info("listMongoDB", "quals", quals)

		client, err := connect(ctx, *GetConfig(d.Connection).ConnectionString)
		if err != nil {
			return nil, err
		}
		dbName := GetConfig(d.Connection).Database

		coll := client.Database(dbName).Collection(collName)
		filter := qualsToMongoFilter(ctx, quals, d.Table.Columns)
		opts := options.Find()
		if d.QueryContext.Limit != nil {
			opts.SetLimit(*d.QueryContext.Limit)
		}
		plugin.Logger(ctx).Info("listMongoDB", "database", dbName, "collection", collName, "filter", filter, "limit", opts.Limit)
		cursor, err := coll.Find(ctx, filter, opts)
		if err != nil {
			return nil, err
		}
		defer cursor.Close(ctx)

		for cursor.Next(ctx) && d.RowsRemaining(ctx) > 0 {
			var result bson.M // A new result variable should be declared for each document.
			if err := cursor.Decode(&result); err != nil {
				return nil, err
			}
			d.StreamListItem(ctx, result)
		}

		return nil, nil
	}
}
