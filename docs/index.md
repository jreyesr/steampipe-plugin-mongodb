---
organization: jreyesr
category: [ "software development" ]
icon_url: "/images/plugins/jreyesr/mongodb.svg"
brand_color: "#00ED64"
display_name: MongoDB
name: mongodb
description: Steampipe plugin for querying MongoDB-stored collections.
og_description: Query MongoDB databases with SQL! Open source CLI. No DB required.
og_image: "/images/plugins/jreyesr/mongodb-social-graphic.png"
engines: [ "steampipe" ]
---

# MongoDB + Steampipe

[Steampipe](https://steampipe.io) is an open-source zero-ETL engine to instantly query cloud APIs using SQL.

[MongoDB Atlas](https://www.mongodb.com/atlas) is a document-oriented (NoSQL) database program.

This plugin provides access, from within Steampipe (a Postgres-powered database), to datasets that are stored in MongoDB
servers.
It dynamically scans the MongoDB collections, and exposes one Steampipe table for each collection. Columns and data
types
are also dynamically detected based on a scan of the data that is stored in the MongoDB collections.

Example query (assuming a MongoDB database that
has [the example `sample_analytics` dataset](https://www.mongodb.com/docs/atlas/sample-data/sample-analytics/#std-label-sample-analytics)
loaded:

```sql
select 
   _id,
   username,
   name,
   address,
   birthdate,
   email,
   accounts
from mongodb.customers;
```

```
+--------------------------+----------------------+-------------------------+------------------------------------+---------------------------+-------------------------------+---------------------------------------------+
| _id                      | username             | name                    | address                            | birthdate                 | email                         | accounts                                    |
+--------------------------+----------------------+-------------------------+------------------------------------+---------------------------+-------------------------------+---------------------------------------------+
| 5ca4bbcea2dd94ee58162a6d | gregoryharrison      | Natalie Ford            | 17677 Mark Crest                   | 1996-09-13T12:14:27-05:00 | amyholland@yahoo.com          | [904260,565468]                             |
|                          |                      |                         | Walterberg, IA 39017               |                           |                               |                                             |
| 5ca4bbcea2dd94ee58162a77 | johnsonshelly        | Jacqueline Haynes       | USNS Howard                        | 1982-09-01T02:12:57-05:00 | virginia36@hotmail.com        | [631901,814687]                             |
|                          |                      |                         | FPO AP 30863                       |                           |                               |                                             |
| 5ca4bbcea2dd94ee58162a69 | valenciajennifer     | Lindsay Cowan           | Unit 1047 Box 4089                 | 1994-02-19T18:46:27-05:00 | cooperalexis@hotmail.com      | [116508]                                    |
|                          |                      |                         | DPO AA 57348                       |                           |                               |                                             |
| 5ca4bbcea2dd94ee58162a74 | patricia44           | Dr. Angela Brown        | 2129 Joel Rapids                   | 1977-06-19T15:35:52-05:00 | michaelespinoza@gmail.com     | [571880]                                    |
|                          |                      |                         | Lisahaven, NE 08609                |                           |                               |                                             |
| 5ca4bbcea2dd94ee58162a68 | fmiller              | Elizabeth Ray           | 9286 Bethany Glens                 | 1977-03-01T21:20:31-05:00 | arroyocolton@gmail.com        | [371138,324287,276528,332179,422649,387979] |
|                          |                      |                         | Vasqueztown, CO 22939              |                           |                               |                                             |
+--------------------------+----------------------+-------------------------+------------------------------------+---------------------------+-------------------------------+---------------------------------------------+
```

## Documentation

- **[Table definitions & examples →](/plugins/jreyesr/mongodb/tables)**

## Get started

### Install

Download and install the latest MongoDB plugin

```bash
steampipe plugin install mongodb
```

### Configuration

Installing the latest mongodbatlas plugin will create a config file (`~/.steampipe/config/mongodb.spc`) with a
single connection named `mongodb`:

```hcl
connection "mongodb" {
  plugin = "jreyesr/mongodb"

  # A connection string (https://www.mongodb.com/docs/drivers/go/current/fundamentals/connections/connection-guide/#connection-uri),
  # in the form that is expected by the official MongoDB Go driver.
  # Can also be set with the `DATABASE_URL` environment variable.
  # Required.
  # connection_string = "mongodb://username:password@localhost:27017/?appName=Steampipe"

  # The MongoDB database that this plugin will expose.
  # Required.
  # database = "dbname"

  # List of collections that will be exposed from the remote DB. No dynamic tables will be created if this arg is empty or not set.
  # Wildcard based searches are supported.
  # For example:
  #  - "*" will expose every collection in the remote DB
  #  - "auth-*" will expose collections whose names start with "auth-"
  #  - "users" will only expose the specific collection "users"
  # You can have several items (for example, ["auth-*", "users"] will expose
  # all the collection that start with "auth-", PLUS the collection "users")
  # Defaults to all collections.
  # collections_to_expose = ["*"]

  # Controls how many documents will be (randomly) sampled from each collection to infer the type of each field.
  # Larger numbers are slower but have a better chance of catching infrequent fields or fields that have a different type on a few documents.
  # Set to 0 to disable sampling and build the types from ALL the documents in the collection. Use with care on large collections!
  # Optional. Defaults to sampling 1000 documents, like MongoDB Compass does on the Schema tab: https://www.mongodb.com/docs/compass/current/sampling/#sampling
  # sample_size = 1000

  # Fields included here won't be analyzed for subfields (instead, the entire field will be presented as a single JSONB column)
  # Useful to stop the plugin from unnecessarily expanding nested documents where the _keys_ are entity IDs or other
  # highly-variable identifiers.
  # For example, {name: {first: "John", last: "Doe"}} should probably be expanded to TWO columns, "name.first" and "name.last",
  # but {reactions: {"user_123": "+1", "user_456": "-1", "user_789": "eyes"}} (e.g. on a Github comment or Slack message)
  # should probably be presented as a single column "reactions" of type JSONB, instead of being exploded to
  # reactions.user_123, reactions.user_456 and reactions.user_789 of type TEXT (since this would create an unbound amount of columns)
  # The format of each item is "collection:path.to.field" (for example, "messages:reactions" if "reactions" is a top-level field on the "messages" collection)
  # Optional. Defaults to analyzing all fields and subfields on all collections (i.e. no fields are skipped)
  # fields_to_ignore = ["collection:path.to.subfield"]
}
```

You must set the `connection_string` field (or, alternatively, set the `DATABASE_URL` environment variable)
to a standard [MongoDB connection string](https://www.mongodb.com/docs/manual/reference/connection-string/).

You must also set the `database` field to the name of the MongoDB database to read. Only one database can be read at a
time. If you need to expose several databases in Steampipe, use
several [connections](https://steampipe.io/docs/reference/config-files/connection) of the same plugin.

Optional settings:

* `collections_to_expose` is a list of collections that will be converted to tables. By default, all collections in the
  database will be included. If values are provided here, only collections whose names match one of the patterns in this
  field will be included. For example, if one of the items is `auth-*`, collections `auth-users` and `auth-sessions`
  will be exposed
* `sample_size` (defaults to 1000) controls how many random documents will be read from each collection to compose the
  schema (i.e. the types for each field) for that collection.
* `fields_to_ignore` can be used if a collection has a nested subdocument whose _keys_ are IDs or other variable data.
  Normally, the plugin will flatten or "explode" nested documents into period-separated columns (for example, the
  document `{category: {name: "General", id: 1}}` will be converted into two columns, `category.name` of type TEXT
  and `category.ID` of type INT). However, if the _keys_ of the document are variable (
  e.g. `{reactions: {user_1: "+1", user_2: "-1", user_3: "confetti"}}`, depending on which users reacted to the entity)
  that would cause an ever-growing number of columns, `reactions.user_1`, `reactions.user_2`, `reactions.user_3`, and so
  on. In such cases, add the subdocument with variable keys to the `fields_to_ignore` list in the
  format `collection:path.to.field`, so the schema analyzer doesn't analyze its contents