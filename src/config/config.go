package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
)

const APP_ROOT_PATH = "/data1/htdocs/"

type Config struct {
	Mq       string `json:"mq"`
	Method   string `json:"method"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Db       int    `json:"db"`
	Auth     string `json:"auth"`
	Max_work int    `json:"max_work"`
	Topic    string `json:"topic"`
	Route    Route  `json:"route"`
}
type Route struct {
	Module     string `json:"module"`
	Controller string `json:"controller"`
	Action     string `json:"action"`
}

// 获取项目名称
func GetAppName() string {
	return os.Getenv("APP_NAME")
}

// 加载配置
// config_file string 配置文件绝对路径
func LoadConfig(config_file string) map[string]Config {
	config_bytes, err := ioutil.ReadFile(config_file)
	if err != nil {
		log.Fatal("config file read err:", err.Error())
	}
	var configs map[string]Config
	err = json.Unmarshal(config_bytes, &configs)
	if err != nil {
		log.Fatal("json decoding faild err:=", err.Error())
	}

	return configs
}
