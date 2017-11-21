/*
 * Copyright (c) 2017-2018 Miguel Ángel Ortuño.
 * See the COPYING file for more information.
 */

package config

type Logger struct {
	Level   string `yaml:"level"`
	LogFile string `yaml:"log_path"`
}