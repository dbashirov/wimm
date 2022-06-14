package config

type StorageConfig struct {
	Host     string
	Port     string
	Database string
	Username string
	Password string
}

type Config struct {
	Storage StorageConfig
}
