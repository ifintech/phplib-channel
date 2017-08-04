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

//加载配置
//app_name string 应用名称
func LoadConfig(app_name string) map[string]Config {
	env := getEnv()

	config_bytes, err := ioutil.ReadFile(APP_ROOT_PATH + app_name + "/conf/server/" + env + "/config.json")
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

func getEnv() string {
	env := os.Getenv("RUN_ENV")
	if "" == env {
		env = "dev"
	}

	return env
}
