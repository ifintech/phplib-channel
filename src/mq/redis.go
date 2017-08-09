package mq

import (
	"config"
	"github.com/garyburd/redigo/redis"
	"strconv"
	"strings"
	"sync"
)

var conn_pool = make(map[string]redis.Conn)
var locks = sync.RWMutex{}

//连接redis
func connect(name string, config config.Config) (redis.Conn, error) {
	locks.Lock()
	defer locks.Unlock()

	if conn, ok := conn_pool[name]; ok {
		return conn, nil
	}

	conn, err := redis.Dial("tcp", config.Dsn.Host+":"+strconv.Itoa(config.Dsn.Port))
	if err != nil {
		return nil, err
	}

	if config.Dsn.Auth != "" {
		_, err = conn.Do("AUTH", config.Dsn.Auth)
		if err != nil {
			return nil, err
		}
	}
	if config.Dsn.Db != 0 {
		_, err = conn.Do("SELECT", config.Dsn.Db)
		if err != nil {
			return nil, err
		}
	}

	if nil == err {
		conn_pool[name] = conn
	}

	return conn, err
}

func RedisPop(name string, config config.Config) (string, error) {
	conn, err := connect(name, config)
	if err != nil {
		return "", err
	}

	data, err := redis.Bytes(conn.Do("lpop", config.Topic))
	if err != nil {
		if strings.Contains(err.Error(), "nil returned") {
			return "", nil
		}
		conn.Close()
		delete(conn_pool, name)

		return "", err
	}

	return string(data), err
}
