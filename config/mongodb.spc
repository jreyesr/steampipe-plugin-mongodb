connection "mongodb" {
  plugin = "jreyesr/mongodb"

  # A connection string (https://www.mongodb.com/docs/drivers/go/current/fundamentals/connections/connection-guide/#connection-uri),
  # in the form that is expected by the official MongoDB Go driver.
  # Can also be set with the `DATABASE_URL` environment variable.
  # Required.
  # connection_string = "mongodb+srv://readonly:kkdkPZQ9snkF74wE@steampipe.ymuibkv.mongodb.net/?appName=Steampipe"

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