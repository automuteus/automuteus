module github.com/denverquane/amongusdiscord

go 1.15

// +heroku goVersion go1.15

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/automuteus/galactus v1.2.2
	github.com/automuteus/utils v0.0.10
	github.com/bsm/redislock v0.7.0
	github.com/bwmarrin/discordgo v0.22.1
	github.com/georgysavva/scany v0.2.7
	github.com/go-redis/redis/v8 v8.4.10
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/jackc/pgx/v4 v4.10.1
	github.com/joho/godotenv v1.3.0
	github.com/nicksnyder/go-i18n/v2 v2.1.2
	github.com/prometheus/client_golang v1.9.0
	golang.org/x/crypto v0.0.0-20201112155050-0c6587e931a9 // indirect
	golang.org/x/text v0.3.5
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/protobuf v1.25.0 // indirect
)

replace github.com/bwmarrin/discordgo v0.22.0 => github.com/automuteus/discordgo v0.22.1