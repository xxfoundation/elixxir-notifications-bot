module gitlab.com/elixxir/notifications-bot

go 1.13

require (
	cloud.google.com/go v0.55.0 // indirect
	cloud.google.com/go/firestore v1.1.1 // indirect
	cloud.google.com/go/pubsub v1.3.1 // indirect
	firebase.google.com/go v3.12.0+incompatible
	github.com/golang/protobuf v1.4.3
	github.com/mitchellh/go-homedir v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.0.0
	github.com/spf13/jwalterweatherman v1.1.0

	github.com/spf13/viper v1.7.0
	gitlab.com/elixxir/comms v0.0.4-0.20210414204605-30cbeab25aaa
	gitlab.com/elixxir/crypto v0.0.7-0.20210412231025-6f75c577f803
	gitlab.com/xx_network/comms v0.0.4-0.20210409202820-eb3dca6571d3
	gitlab.com/xx_network/crypto v0.0.5-0.20210405224157-2b1f387b42c1
	gitlab.com/xx_network/primitives v0.0.4-0.20210402222416-37c1c4d3fac4
	golang.org/x/net v0.0.0-20201029221708-28c70e62bb1d
	golang.org/x/tools v0.0.0-20200318150045-ba25ddc85566 // indirect
	google.golang.org/api v0.20.0
	google.golang.org/genproto v0.0.0-20201030142918-24207fddd1c3 // indirect
	google.golang.org/grpc v1.33.1
	gopkg.in/ini.v1 v1.56.0 // indirect
	gorm.io/driver/postgres v1.0.8
	gorm.io/gorm v1.20.12
)
