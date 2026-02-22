module github.com/Abdullah4AI/apple-toolkit

go 1.26.0

require (
	github.com/Abdullah4AI/appstore v0.0.0
	github.com/Abdullah4AI/swiftship v0.0.0
	github.com/spf13/cobra v1.8.1
)

require (
	github.com/99designs/go-keychain v0.0.0-20191008050251-8e49817e8af4 // indirect
	github.com/99designs/keyring v1.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.6.2 // indirect
	github.com/clipperhouse/stringish v0.1.1 // indirect
	github.com/clipperhouse/uax29/v2 v2.3.0 // indirect
	github.com/danieljoos/wincred v1.2.3 // indirect
	github.com/dvsekhvalnov/jose2go v1.8.0 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/godbus/dbus v0.0.0-20190726142602-4481cbc300e2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/google/jsonschema-go v0.4.2 // indirect
	github.com/gsterjov/go-libsecret v0.0.0-20161001094733-a6f4afe4910c // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.19 // indirect
	github.com/modelcontextprotocol/go-sdk v1.3.1 // indirect
	github.com/mtibben/percent v0.2.1 // indirect
	github.com/olekukonko/cat v0.0.0-20250911104152-50322a0618f6 // indirect
	github.com/olekukonko/errors v1.1.0 // indirect
	github.com/olekukonko/ll v0.1.4-0.20260115111900-9e59c2286df0 // indirect
	github.com/olekukonko/tablewriter v1.1.3 // indirect
	github.com/peterbourgon/ff/v3 v3.4.0 // indirect
	github.com/segmentio/asm v1.1.3 // indirect
	github.com/segmentio/encoding v0.5.3 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/tidwall/jsonc v0.3.2 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	go.mozilla.org/pkcs7 v0.9.0 // indirect
	golang.org/x/mod v0.32.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/term v0.40.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	howett.net/plist v1.0.1 // indirect
)

replace (
	github.com/Abdullah4AI/appstore => github.com/Abdullah4AI/appstore v0.0.0
	github.com/Abdullah4AI/swiftship => github.com/Abdullah4AI/swiftship v0.0.0
)

// For local development, override with:
//   go.work or -replace flag
// replace github.com/Abdullah4AI/appstore => ../appstore
// replace github.com/Abdullah4AI/swiftship => ../swiftship
