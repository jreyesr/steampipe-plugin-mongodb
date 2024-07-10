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
}