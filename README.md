# MongoDB Atlas Plugin for Steampipe

Use SQL to query data that is stored on collections in MongoDB databases.

- **[Get started →](https://hub.steampipe.io/plugins/jreyesr/mongodb)**
- Documentation: [Table definitions & examples](https://hub.steampipe.io/plugins/jreyesr/mongodb/tables)
- Community: [Join #steampipe on Slack →](https://turbot.com/community/join)
- Get involved: [Issues](https://github.com/jreyesr/steampipe-plugin-mongodb/issues)

## Quick start

Install the plugin with [Steampipe](https://steampipe.io):

```shell
steampipe plugin install jreyesr/mongodb
```

Set up a MongoDB database. If you don't have one yet, use [MongoDB Atlas's sample data](https://www.mongodb.com/docs/atlas/sample-data/#std-label-load-sample-data).

Run a query (here we're using [the `sample_analytics` database](https://www.mongodb.com/docs/atlas/sample-data/sample-analytics/#std-label-sample-analytics):

```sql
select
  *
from
  mongodb.transactions
where
  transaction_count < 10 
```

## Engines

This plugin is available for the following engines:

| Engine        | Description
|---------------|------------------------------------------
| [Steampipe](https://steampipe.io/docs) | The Steampipe CLI exposes APIs and services as a high-performance relational database, giving you the ability to write SQL-based queries to explore dynamic data. Mods extend Steampipe's capabilities with dashboards, reports, and controls built with simple HCL. The Steampipe CLI is a turnkey solution that includes its own Postgres database, plugin management, and mod support.

## Developing

Prerequisites:

- [Steampipe](https://steampipe.io/downloads)
- [Golang](https://golang.org/doc/install)

Clone:

```sh
git clone https://github.com/jreyesr/steampipe-plugin-mongodb.git
cd steampipe-plugin-mongodb
```

Build, which automatically installs the new version to your `~/.steampipe/plugins` directory:

```
make
```

Configure the plugin:

```
cp config/* ~/.steampipe/config
vi ~/.steampipe/config/mongodb.spc
```

Try it!

```
steampipe query
> .inspect mongodb
```

Further reading:

- [Writing plugins](https://steampipe.io/docs/develop/writing-plugins)
- [Writing your first table](https://steampipe.io/docs/develop/writing-your-first-table)

## Open Source & Contributing

This repository is published under the [Apache 2.0](https://www.apache.org/licenses/LICENSE-2.0) (source code) and [CC BY-NC-ND](https://creativecommons.org/licenses/by-nc-nd/2.0/) (docs) licenses. Please see our [code of conduct](https://github.com/turbot/.github/blob/main/CODE_OF_CONDUCT.md). We look forward to collaborating with you!

[Steampipe](https://steampipe.io) is a product produced from this open source software, exclusively by [Turbot HQ, Inc](https://turbot.com). It is distributed under our commercial terms. Others are allowed to make their own distribution of the software, but cannot use any of the Turbot trademarks, cloud services, etc. You can learn more in our [Open Source FAQ](https://turbot.com/open-source).

## Get Involved

**[Join #steampipe on Slack →](https://turbot.com/community/join)**

Want to help but don't know where to start? Pick up one of the `help wanted` issues:

- [Steampipe](https://github.com/turbot/steampipe/labels/help%20wanted)
- [MongoDB Plugin](https://github.com/jreyesr/steampipe-plugin-mongodb/labels/help%20wanted)