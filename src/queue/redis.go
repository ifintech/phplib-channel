package queue

import (
	"github.com/garyburd/redigo/redis"
	"fmt"
	"strings"
	"config"
)

type Redis struct {
	Base
	conn redis.Conn
}

//连接redis
func getRedisInstance(config config.Config) (Queue, error) {
	conn, err := redis.Dial("tcp", config.Host + ":" + fmt.Sprint(config.Port))
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

	return Redis{Base{config}, conn}, err
}

func (queue Redis)Pop() (string, error) {
	data, err := redis.Bytes(queue.conn.Do("lpop", queue.config.Topic))
	if (nil != err) {
		if (strings.Contains(err.Error(), "nil returned")) {
			return "", nil
		}

		return "", err
	}

	return string(data), err
}
func (queue Redis)Close() {
	queue.conn.Close()
}