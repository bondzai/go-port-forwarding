package model

import "sync"

type Mapping struct {
	Remote string `yaml:"remote"`
	Local  string `yaml:"local"`
}

type Config struct {
	Mappings []Mapping `yaml:"mappings"`
}

var ConfigFile = "config.yaml"
var ShutdownCh = make(chan struct{})

var Once sync.Once

var BufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 4096)
	},
}
