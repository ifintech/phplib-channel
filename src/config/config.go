package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

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
func LoadConfig() map[string]Config {
	config_bytes, err := ioutil.ReadFile(GetCurrentDirectory() + "/config.json")
	if err != nil {
		log.Fatal("config file read err: ", err.Error())
	}
	var configs map[string]Config
	err = json.Unmarshal(config_bytes, &configs)
	if err != nil {
		log.Fatal("json decoding faild err: ", err.Error())
	}

	return configs
}

func GetParentDirectory(directory string) string {
	return subStr(directory, 0, strings.LastIndex(directory, "/"))
}

func GetCurrentDirectory() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	return strings.Replace(dir, "\\", "/", -1)
}

func subStr(s string, pos, length int) string {
	runes := []rune(s)
	l := pos + length
	if l > len(runes) {
		l = len(runes)
	}
	return string(runes[pos:l])
}
