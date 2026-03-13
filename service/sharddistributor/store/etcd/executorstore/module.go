package executorstore

import "go.uber.org/fx"

var Module = fx.Module("executorstore",
	fx.Provide(NewStore),
	fx.Provide(NewClient),
	fx.Provide(NewETCDConfig),
)
