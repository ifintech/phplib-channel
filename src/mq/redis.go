package mq

import (
	"github.com/garyburd/redigo/redis"
	"strconv"
	"strings"
)

//连接redis
func connect(config Config) (interface{}, error) {
	conn, err := redis.Dial("tcp", config.Dsn.Host+":"+strconv.Itoa(config.Dsn.Port))
	if err != nil {
		return nil, err
	}

	if config.Auth != "" {
		_, err = conn.Do("AUTH", config.Auth)
		if err != nil {
			return nil, err
		}
	}
	if config.Db != 0 {
		_, err = conn.Do("SELECT", config.Db)
		if err != nil {
			return nil, err
		}
	}

	return conn, err
}

func RedisPop(config Config) (data string, err error) {
	conn, err := connect(config)
	if err != nil {
		return nil, err
	}
	data, err = redis.Bytes(conn.Do("lpop", config.Topic))
	if err != nil {
		if strings.Contains(err.Error(), "nil returned") {
			return "", nil
		}
		return nil, err
	}

	return string(data), err
}