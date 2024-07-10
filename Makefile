install:
	go build -o ~/.steampipe/plugins/hub.steampipe.io/plugins/jreyesr/mongodb@latest/steampipe-plugin-mongodb.plugin -gcflags="all=-N -l" *.go