package log

// Config is a configuration of log.Logger.
type Config struct {
	// Stdout is true then the output needs to goto standard out
	// By default this is false and output will go to standard error
	Stdout bool `yaml:"stdout"`
	// Level is the desired log level
	Level string `yaml:"level"`
	// OutputFile is the path to the log output file
	// Stdout must be false, otherwise Stdout will take precedence
	OutputFile string `yaml:"outputFile"`
	// LevelKey is the desired log level, defaults to "level"
	LevelKey string `yaml:"levelKey"`
	// Encoding decides the format, supports "console" and "json".
	// "json" will print the log in JSON format(better for machine), while "console" will print in plain-text format(more human friendly)
	// Default is "json"
	Encoding string `yaml:"encoding"`
}
