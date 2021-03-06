/*
 * Copyright (c) 2018 Miguel Ángel Ortuño.
 * See the LICENSE file for more information.
 */

package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type Debug struct {
	Port int `yaml:"port"`
}

type Config struct {
	PIDFile string   `yaml:"pid_path"`
	Debug   *Debug   `yaml:"debug"`
	Logger  Logger   `yaml:"logger"`
	Storage Storage  `yaml:"storage"`
	C2S     C2S      `yaml:"c2s"`
	Servers []Server `yaml:"servers"`
}

var DefaultConfig Config

func Load(configFile string) error {
	b, err := ioutil.ReadFile(configFile)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(b, &DefaultConfig)
}
