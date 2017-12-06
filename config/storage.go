/*
 * Copyright (c) 2017-2018 Miguel Ángel Ortuño.
 * See the COPYING file for more information.
 */

package config

import (
	"fmt"
)

type StorageType int

const (
	MySQL StorageType = iota
)

type Storage struct {
	Type  StorageType
	MySQL MySQLDb
}

type MySQLDb struct {
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	PoolSize int    `yaml:"pool_size"`
}

type storageProxyType struct {
	Type  string  `yaml:"type"`
	MySQL MySQLDb `yaml:"mysql"`
}

func (s *Storage) UnmarshalYAML(unmarshal func(interface{}) error) error {
	p := storageProxyType{}
	unmarshal(&p)
	switch p.Type {
	case "mysql":
		s.Type = MySQL
	default:
		return fmt.Errorf("config.Storage: unrecognized storage type: %s", p.Type)
	}
	s.MySQL = p.MySQL
	return nil
}
