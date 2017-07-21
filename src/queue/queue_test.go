package queue

import (
	"fmt"
	"testing"
	"io/ioutil"
	"log"
	"encoding/json"
	"config"
)

func TestPopRedis(t *testing.T) {
	config_bytes, err := ioutil.ReadFile("/data1/htdocs/cgdemo/bin/config.json")
	if err != nil {
		log.Fatal("config file not exist err:", err.Error())
	}
	var configs map[string]config.Config
	err = json.Unmarshal(config_bytes, &configs)
	if err != nil {
		log.Fatal("json decoding faild err:", err.Error())
	}

	for name, config := range configs{
		queue, err := GetInstance(name, config)
		if (nil != err) {
			fmt.Println(err)
			continue
		}

		msg, err := queue.Pop()
		fmt.Println(msg, err)

		queue.Close()
	}
}