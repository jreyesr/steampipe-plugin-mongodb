---
title: "Steampipe Table: {collection_name} - Query MongoDB collections using SQL"
description: "Allows users to query MongoDB collections, specifically to extract, transform, and load data from MongoDB collections for analysis and reporting."
---

# Table: {collection_name} - Query MongoDB collections using SQL

MongoDB collections are stores for unstructured data (similar to JSON documents, but with extra data types in addition
to strings,
numbers, and booleans). Unlike relational databases, MongoDB collections are commonly used to store denormalized data,
where documents are directly embedded into other documents, as opposed to being linked via foreign keys.

## Table Usage Guide

The `collection_name` table provides insights into the data stored within mongoDB collections. As a data analyst, you
can leverage this table to extract, transform, and load data from MongoDB collections for in-depth analysis and
reporting, or to merge with other [data sources](https://hub.steampipe.io/#search). Use it to uncover valuable insights
from your data, such as identifying trends, patterns, and correlations.

Schema
link: [https://hub.steampipe.io/plugins/jreyesr/mongodb/tables/collection_name](https://hub.steampipe.io/plugins/jreyesr/mongodb/tables/%7Bcollection_name%7D)

Query data from MongoDB collections. A table is automatically created to represent each
MongoDB collection found in the configured `collections_to_expose`.

For instance, if `collections_to_expose` is set to `customers`, and the collection `customers` in MongoDB contains:

```json lines
{
  "_id": ObjectID("5ca4bbcea2dd94ee58162a6d"),
  "username": "alice",
  "name": "Alice",
  "preferences": {
    "tutorial": false,
    "marketing_emails": false
  }
}
{
  "_id": ObjectID("5ca4bbcea2dd94ee58162a68"),
  "username": "bob",
  "name": "Bob",
  "preferences": {
    "tutorial": true,
    "marketing_emails": false
  }
}
```

This plugin will create 1 table `users`, which you can then query directly:

```sql
select _id,
       username,
       name,
       "preference.tutorial",
       "preferences.marketing_emails"
from mongodb.customers;
```

```
+--------------------------+----------+-------+----------------------+------------------------------+
| _id                      | username | name  | preferences.tutorial | preferences.All column values are returned as text data type.
 |
+--------------------------+----------+-------+----------------------+------------------------------+
| 5ca4bbcea2dd94ee58162a6d | alice    | Alice | false                | false                        |
| 5ca4bbcea2dd94ee58162a68 | bob      | Bob   | true                 | false                        |
+--------------------------+----------+-------+----------------------+------------------------------+
```

The types of the columns are automatically detected from the source MongoDB data, by randomly sampling a set of
documents (1000 by default) and choosing an appropriate type. For instance, if the field `username` is a string on all
MongoDB documents, then the `username` column will be of type TEXT.

* If a field has always the same type in the MongoDB document, then the corresponding Postgres column will have a type
  derived from that MongoDB type
    * `TEXT`: Will be generated for Mongo strings and other string-like types, such as ObjectIDs, binary data, UUIDs,
      and JS code
    * `INT`: Will be used for Mongo Int32 and Int46 fields
    * `DOUBLE`: Will be used for MongoDB Double and Decimal128
    * `JSONB`: Will be used for DBRefs and Regex objects (the latter in the form `{pattern: "regex.*", flags: "i"}`)
    * `TIMESTAMP`: Will be used for Mongo's Timestamps and DateTime
    * `BOOLEAN`: Will be used for MongoDB's boolean values
* Fields that are arrays in MongoDB will always have type `JSONB`
* Nested subdocuments in MongoDB will be exploded into their subfields (see `preferences.tutorial`
  and `preferences.marketing_emails` above)
* Fields that have been observed to have several types (for instance, a field that sometimes is a string and sometimes
  an array of strings) will have type `JSONB`, since JSONB can contain multiple types

## Examples

**Note**: All examples in this section assume the `database` configuration argument is set to `sample_analytics` and the
plugin is pointing to a MongoDB database
with [the `sample_analytics` dataset loaded](https://www.mongodb.com/docs/atlas/sample-data/sample-analytics/#std-label-sample-analytics).
For more information on how column names are created, please
see [Column Names](https://hub.steampipe.io/plugins/jreyesr/mongodb/tables/{collection_name}#column-names).

### Inspect the table structure

Assuming your connection is called `mongodb` (the default), list all tables with:

```bash
.inspect mongodb
+--------------+------------------------------------------------------+
| table        | description                                          |
+--------------+------------------------------------------------------+
| accounts     | Collection accounts on database sample_analytics     |
| customers    | Collection customers on database sample_analytics    |
| transactions | Collection transactions on database sample_analytics |
+--------------+------------------------------------------------------+
```

To get details for a specific table, inspect it by name:

```bash
.inspect mongodb.customers
+--------------------+--------------------------+---------------------------------+
| column             | type                     | description                     |
+--------------------+--------------------------+---------------------------------+
| _id                | text                     | Field _id                       |
| accounts           | jsonb                    | Field accounts                  |
| address            | text                     | Field address                   |
| birthdate          | timestamp with time zone | Field birthdate                 |
| email              | text                     | Field email                     |
| name               | text                     | Field name                      |
| tier_and_details   | jsonb                    | Field tier_and_details          |
| username           | text                     | Field username                  |
+--------------------+--------------------------+---------------------------------+
```

### Query a collection

To retrieve all the information in a collection, with no filters, run a `SELECT` query with no `WHERE` clause.
Given the collection `customers`, the query is:

```sql+postgres
select
  *
from
  mongodb.customers;
```

### Query specific columns

Determine the areas in which you want to focus by selecting specific fields. This is useful when you want to narrow down your data analysis to specific attributes.
The types of the columns depend on the schema analysis that is performed when the plugin loads. The column names come from the names of the fields.

```sql+postgres
select
  _id,
  name,
  email,
  birthdate
from
  mongodb.customers
LIMIT 10;
```

If your column names are complex (e.g. they contain spaces or periods), use identifier quotes:

```sql+postgres
select
  _id,
  "preferences.email"
from
  mongodb.customers;
```

### Retrieve a specific item

If you know the ID of a specific item (which is, by convention, stored on the `_id` field 
and is usually an ObjectID), you can add a `WHERE` condition:

```sql+postgres
select
  *
from
  mongodb.customers
where
  _id='5ca4bbc7a2dd94ee5816238d';
```

### Filter columns

Some filters can be pushed down to the MongoDB datastore. For example, to filter for customers born after the year 1990:

```sql+postgres
select
  _id,
  name,
  email,
  birthdate
from
  mongodb.customers
where
  birthdate > '1990-01-01';
```

## Column Names

The column names are derived from the fields that appear in the documents that are stored in that MongoDB collection. 
For example, if a collection `customers` contains documents that look like this:

```json
{
  "_id": ObjectID("5ca4bbcea2dd94ee58162a6d"),
  "username": "gregoryharrison",  
  "name": "Natalie Ford",  
  "address": "17677 Mark Crest\nWalterberg, IA 39017", 
  "birthdate": ISODate("1996-09-13T17:14:27.000+00:00"),
  "email": "amyholland@yahoo.com",
  "accounts": [904260, 565468],  
  "preferences": {
    "tutorial": false,
    "marketing_emails": false
  }
}
```

The MongoDB plugin will create a table called `customers` with the field names as column names:

```bash
.inspect mongodb.customers
+------------------------------+--------------------------+------------------------------------+
| column                       | type                     | description                        |
+------------------------------+--------------------------+------------------------------------+
| _id                          | text                     | Field _id                          |
| accounts                     | jsonb                    | Field accounts                     |
| address                      | text                     | Field address                      |
| birthdate                    | timestamp with time zone | Field birthdate                    |
| email                        | text                     | Field email                        |
| name                         | text                     | Field name                         |
| preferences.tutorial         | boolean                  | Field preferences.tutorial         |
| preferences.marketing_emails | boolean                  | Field preferences.marketing_emails |
| username                     | text                     | Field username                     |
+------------------------------+--------------------------+------------------------------------+
```
