package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
)

type Config struct {
	Mq       string `json:"mq"`
	Method   string `json:"method"`
	App_name string `json:"app_name"`
	Dsn      Dsn    `json:"dsn"`
	Max_work int    `json:"max_work"`
	Topic    string `json:"topic"`
	Uri      string `json:"uri"`
}

type Dsn struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Db       int    `json:"db"`
	Auth     string `json:"auth"`
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
		log.Fatal("json decoding faild err:=", err.Error())
	}

	return configs
}
