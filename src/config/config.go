package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
)

type Config struct {
	Mq       string `json:"mq"`
	Method   string `json:"method"`
	Max_work int    `json:"max_work"`
	Topic    string `json:"topic"`
	Uri      string `json:"uri"`
	Dsn      Dsn    `json:"dsn"`
}

type Dsn struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	Db   int    `json:"db"`
	Auth string `json:"auth"`
}

func GetAppName() string {
	return os.Getenv("APP_NAME")
}

//加载配置
//app_name string 应用名称
func LoadConfig(config_path string) map[string]Config {
	config_bytes, err := ioutil.ReadFile(config_path)
	if err != nil {
		log.Fatal("config file read err:", err.Error())
	}
	var configs map[string]Config
	err = json.Unmarshal(config_bytes, &configs)
	if err != nil {
		log.Fatal("json decoding faild err:", err.Error())
	}

	return configs
}
