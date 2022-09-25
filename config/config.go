package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

type StorageConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Database string `yaml:"database"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type Server struct {
	Port string `yaml:"port"`
}

type JWT struct {
	SecretKey string `yaml:"secretKey"`
}

type Session struct {
	SessionKey string `yaml:"sessionKey"`
}

type Config struct {
	Storage StorageConfig `yaml:"storage"`
	Server  Server        `yaml:"server"`
	JWT     JWT           `yaml:"JWT"`
	Session Session       `yaml:"session"`
}

// var sc *Config

func GetConfig() (*Config, error) {
	sc := &Config{}
	yamlFile, err := ioutil.ReadFile("config/config.yml")
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(yamlFile, sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}
