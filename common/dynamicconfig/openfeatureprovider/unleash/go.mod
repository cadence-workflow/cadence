module github.com/uber/cadence/common/dynamicconfig/openfeatureprovider/unleash

go 1.25.0

replace github.com/uber/cadence => ../../../..

require (
	github.com/Unleash/unleash-client-go/v4 v4.5.0
	github.com/open-feature/go-sdk v1.17.1
	github.com/open-feature/go-sdk-contrib/providers/unleash v0.1.1-alpha
	github.com/stretchr/testify v1.11.1
	github.com/uber/cadence v0.0.0-00010101000000-000000000000
)

require (
	github.com/Masterminds/semver/v3 v3.3.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	go.uber.org/mock v0.6.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
