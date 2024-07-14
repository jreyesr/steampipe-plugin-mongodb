package mongodb

import (
	"fmt"
	"github.com/turbot/steampipe-plugin-sdk/v5/plugin"
	"github.com/turbot/steampipe-plugin-sdk/v5/plugin/schema"
	"os"
	"strings"
)

type MongoDBConfig struct {
	ConnectionString    *string  `cty:"connection_string"`
	Database            string   `cty:"database"`
	CollectionsToExpose []string `cty:"collections_to_expose"`
	FieldsToIgnore      []string `cty:"fields_to_ignore"`
}

var ConfigSchema = map[string]*schema.Attribute{
	"connection_string":     {Type: schema.TypeString},
	"database":              {Type: schema.TypeString, Required: true},
	"collections_to_expose": {Type: schema.TypeList, Elem: &schema.Attribute{Type: schema.TypeString}},
	"fields_to_ignore":      {Type: schema.TypeList, Elem: &schema.Attribute{Type: schema.TypeString}},
}

func ConfigInstance() interface{} {
	return &MongoDBConfig{}
}

// GetConfig retrieves and casts connection config from query data
func GetConfig(connection *plugin.Connection) MongoDBConfig {
	if connection == nil || connection.Config == nil {
		return MongoDBConfig{}
	}
	config, _ := connection.Config.(MongoDBConfig)
	return config
}

func (c MongoDBConfig) String() string {
	return fmt.Sprintf(
		"MongoDBConfig{database=%s}",
		c.Database) // can't print connection_string, since it has credentials embedded
}

func (c MongoDBConfig) GetConnectionString() (string, error) {
	if c.ConnectionString != nil && *c.ConnectionString != "" {
		return *c.ConnectionString, nil
	}

	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v, nil
	}

	return "", fmt.Errorf("please provide either the connection_string param or the DATABASE_URL envvar")
}

/*
GetCollectionsToExpose returns the slice of collection blobs that was configured in the .spc file, if set, and ["*"] otherwise (which will expose every collection)
*/
func (c MongoDBConfig) GetCollectionsToExpose() []string {
	if len(c.CollectionsToExpose) > 0 {
		return c.CollectionsToExpose
	}
	return []string{"*"}
}

/*
GetFieldsToIgnore returns only those fields in [MongoDBConfig.FieldsToIgnore] that start with "[collection]:".
This is the convention to signal that a certain field path applies to a certain collection
*/
func (c MongoDBConfig) GetFieldsToIgnore(collection string) []string {
	if len(c.FieldsToIgnore) == 0 {
		return []string{}
	}

	// Only take into account ignore fields that start with "<collName>:"
	collectionPrefix := fmt.Sprintf("%s:", collection)
	fieldsForCollection := make([]string, 0)
	for _, field := range c.FieldsToIgnore {
		if strings.HasPrefix(field, collectionPrefix) {
			fieldName, _ := strings.CutPrefix(field, collectionPrefix)
			fieldsForCollection = append(fieldsForCollection, fieldName)
		}
	}
	return fieldsForCollection
}
