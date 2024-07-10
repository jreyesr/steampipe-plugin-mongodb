package mongodb

import (
	"context"
	"fmt"
	"github.com/turbot/go-kit/helpers"
	"github.com/turbot/steampipe-plugin-sdk/v5/plugin"
	"github.com/turbot/steampipe-plugin-sdk/v5/plugin/transform"
	"path"
)

func Plugin(ctx context.Context) *plugin.Plugin {
	p := &plugin.Plugin{
		Name:             "steampipe-plugin-mongodb",
		DefaultTransform: transform.FromGo().NullIfZero(),
		ConnectionConfigSchema: &plugin.ConnectionConfigSchema{
			NewInstance: ConfigInstance,
			Schema:      ConfigSchema,
		},
		SchemaMode:   plugin.SchemaModeDynamic,
		TableMapFunc: PluginTables,
	}
	return p
}

type key string

const (
	keyCollection key = "collection"
)

func PluginTables(ctx context.Context, d *plugin.TableMapData) (map[string]*plugin.Table, error) {
	tables := map[string]*plugin.Table{}

	config := GetConfig(d.Connection)
	connectionString, err := config.GetConnectionString()
	if err != nil {
		return nil, err
	}
	databaseName := config.Database

	collections, err := getCollectionsOnDatabase(ctx, connectionString, databaseName)
	if err != nil {
		plugin.Logger(ctx).Error("mongodb.PluginCollections", "get_collections_error", err)
		return nil, err
	}
	fmt.Printf("connstring %s", connectionString)

	tempCollectionNames := []string{} // this is to keep track of the collections that we've already added

	plugin.Logger(ctx).Debug("mongodb.PluginCollections", "collections", collections, "patterns", config.GetCollectionsToExpose())
	for _, pattern := range config.GetCollectionsToExpose() {
		for _, collection := range collections {
			if helpers.StringSliceContains(tempCollectionNames, collection) {
				continue // we've already handled it before
			} else if ok, _ := path.Match(pattern, collection); !ok {
				plugin.Logger(ctx).Debug("mongodb.PluginCollections.noMatch", "pattern", pattern, "table", collection)
				continue // pattern didn't match, don't do what follows
			}

			// here we're sure that pattern matched and it's the first time, so process this table
			tempCollectionNames = append(tempCollectionNames, collection)

			// Pass the collection name as a context key, as the CSV plugin does with each file path
			// See https://github.com/turbot/steampipe-plugin-csv/blob/cb5bbca5c9fdaa18a03ebd3953dbb0ab501b18bd/csv/plugin.go#L45
			tableCtx := context.WithValue(ctx, keyCollection, collection)

			tableSteampipe, err := tableMongoDB(tableCtx, d.Connection)
			if err != nil {
				plugin.Logger(ctx).Error("mongodb.PluginCollections", "create_table_error", err, "collectionName", collection)
				return nil, err
			}

			plugin.Logger(ctx).Debug("mongodb.PluginCollections.makeTables", "table", tableSteampipe)
			tables[collection] = tableSteampipe
		}
	}

	// Manually add the raw table (that one will always exist, in addition to an unknown number of dynamic tables)
	//tables["raw"] = tableRawQuery(ctx, d.Connection)

	plugin.Logger(ctx).Debug("mongodb.PluginTables.makeTables", "tables", tables)
	return tables, nil
}
