package queue

import "config"

type Mns struct {
	Base
}

//连接mns
func getMnsInstance(config config.Config) (Queue, error) {
	return Mns{Base{config}}, nil
}

func (queue Mns)Pop() (string, error) {
	return "", nil
}
func (queue Mns)Close() {
}