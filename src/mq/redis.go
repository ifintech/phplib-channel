package mq

import (
	"github.com/garyburd/redigo/redis"
	"fmt"
	"config"
)

type Redis struct {
	Base
	conn redis.PubSubConn
}

//连接redis
func getRedisInstance(config config.Config) (Mq, error) {
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

	psc := redis.PubSubConn{conn}
	psc.Subscribe(config.Topic)

	return &Redis{Base{config: config}, psc}, err
}

func (mq *Redis) Sub() (string, error) {
	switch msg := mq.conn.Receive().(type) {
	case redis.Message:
		return string(msg.Data), nil
	case redis.Subscription:
		if msg.Count == 0 {
			return "", nil
		}
	case error:
		return "", msg
	}

	return "", nil
}

func (mq Redis) UnSub()  {
	mq.conn.Unsubscribe(mq.config.Topic)
}

func (mq Redis) Close() {
	mq.conn.Close()
}