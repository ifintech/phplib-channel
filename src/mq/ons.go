package mq

import (
	"config"
)

type Ons struct {
	Base
}

//连接ons
func getOnsInstance(config config.Config) (Mq, error) {
	return nil, nil
}

func (mq Ons) Sub() (string, error) {
	return "", nil
}

func (mq Ons) UnSub() {
}

func (mq Ons) Close() {
}
