module gitlab.com/elixxir/notifications-bot

go 1.13

require (
	cloud.google.com/go/firestore v1.1.1 // indirect
	firebase.google.com/go v3.12.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/sideshow/apns2 v0.20.0
	github.com/spf13/cobra v1.0.0
	github.com/spf13/jwalterweatherman v1.1.0
	github.com/spf13/viper v1.7.0
	gitlab.com/elixxir/comms v0.0.4-0.20220208155528-2fe599a0218d
	gitlab.com/elixxir/crypto v0.0.7-0.20220110170041-7e42f2e8b062
	gitlab.com/elixxir/primitives v0.0.3-0.20220208153511-67470fbdbd1c
	gitlab.com/xx_network/comms v0.0.4-0.20220126231737-fe2338016cce
	gitlab.com/xx_network/crypto v0.0.5-0.20211227194420-f311e8920467
	gitlab.com/xx_network/primitives v0.0.4-0.20211222205802-03e9d7d835b0
	golang.org/x/net v0.0.0-20210525063256-abc453219eb5
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8 // indirect
	google.golang.org/api v0.30.0
	google.golang.org/genproto v0.0.0-20201030142918-24207fddd1c3 // indirect
	gopkg.in/ini.v1 v1.56.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gorm.io/driver/postgres v1.0.8
	gorm.io/gorm v1.20.12
)
