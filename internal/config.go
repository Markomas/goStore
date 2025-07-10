package internal

type Config struct {
	Server struct {
		Port   string `yaml:"port"`
		APIKey string `yaml:"api_key"`
	} `yaml:"server"`
	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`
}
