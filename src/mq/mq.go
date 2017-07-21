package mq

import (
	"config"
	"errors"
	"strings"
	"sync"
)

const TYPE_REDIS = "redis"

type Base struct {
	config config.Config
}

type Mq interface {
	Sub() (string, error)
	UnSub()
	Close()
}

var types = []string{
	TYPE_REDIS,
}
var conn_pool = make(map[string]Mq)
var locks = sync.RWMutex{}

func IsValidType(queue_type string) bool {
	for _, item := range types {
		if item == queue_type {
			return true
		}
	}
	return false
}

func GetInstance(name string, config config.Config) (Mq, error) {
	mq_type := config.Mq
	key := getKey(mq_type, name)

	locks.Lock()
	defer locks.Unlock()

	if instance, ok := conn_pool[key]; ok {
		return instance, nil
	}

	var instance Mq
	var err error

	switch strings.ToLower(mq_type) {
	case TYPE_REDIS:
		instance, err = getRedisInstance(config)
	default:
		return nil, errors.New("invalid type " + mq_type)
	}

	if nil == err {
		conn_pool[key] = instance
	}

	return instance, err
}

func RemoveInstance(mq string, name string) {
	key := getKey(mq, name)

	locks.Lock()
	defer locks.Unlock()

	if conn, ok := conn_pool[key]; ok {
		conn.Close()
		delete(conn_pool, key)
	}
}

func getKey(mq string, name string) string {
	return mq + "_" + name
}
