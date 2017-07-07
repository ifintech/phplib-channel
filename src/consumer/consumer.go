package main

import (
	"runtime"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"sync"
	"fcgiclient"
	"strings"
	"path/filepath"
	"github.com/garyburd/redigo/redis"
	"time"
	"fmt"
)

type config struct {
	Mq	 string `json:"mq"`
	Method	 string `json:"method"`
	Host     string `json:"host"`
	Port     int `json:"port"`
	Db       int `json:"db"`
	Auth     string `json:"auth"`
	Max_work int `json:"max_work"`
	Topic	 string `json:"topic"`
	Route	 route `json:"route"`
}

type route struct {
	Module string `json:"module"`
	Controller string `json:"controller"`
	Action string `json:"action"`
}

var wg sync.WaitGroup
var consumer_wg sync.WaitGroup

func main() {
	//使用上多核
	runtime.GOMAXPROCS(runtime.NumCPU())

	//注册信号
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	//读取配置
	config_bytes, err := ioutil.ReadFile(getCurrentDirectory() + "/config.json")
	if err != nil {
		log.Fatal("config file not exist err:", err.Error())
	}
	var configs map[string]config
	err = json.Unmarshal(config_bytes, &configs)
	if err != nil {
		log.Fatal("json decoding faild err:", err.Error())
	}

	//todo 检查fastcgi服务是否开启

	//每个topic协程需要接受主进程停止信号
	var consumer_signs = make(map[string] (chan int))

	for name, config := range configs {
		consumer_signs[name] = make(chan int, 1)

		go consume(name, config, consumer_signs[name])
	}

	Loop:
	//主进程不退出，直到收到信号退出，同时通知协程停止获取数据，处理完积压数据
	for {
		select {
		case <-sigs:
			for _, sign := range consumer_signs {
				sign <- 0
			}

			break Loop
		}
	}

	//等待协程完成,退出
	wg.Wait()
	log.Println("safe exit");
}

//获取消息队列数据，以channel方式返回
func consume(name string, config config, sign_consumer <- chan int){
	wg.Add(1)
	defer wg.Done()

	//设置最大的请求并发量
	work_num := make(chan int, config.Max_work)

	log.Println(config.Mq, config.Method)
	if(config.Mq == "redis" && config.Method == "lpop"){
		//链接redis todo db&auth
		conn, err := redis.Dial("tcp", config.Host+":"+fmt.Sprint(config.Port))
		if err != nil {
			log.Fatal("mq "+name+" connect err",err.Error())
		}
		defer conn.Close()

		Loop:
		for {
			select {
			case <- sign_consumer:
				consumer_wg.Wait()
				log.Println("break consumer: " + name)

				break Loop
			default:
				consumer_wg.Add(1)
				work_num <- 1

				data, err := redis.Bytes(conn.Do(config.Method, config.Topic))
				if err != nil {
					consumer_wg.Done()
					<-work_num

					log.Println("pop err: ", err.Error())
					time.Sleep(time.Second * 1)
					continue
				}

				log.Println(name + " get data: ", string(data))

				go requestFpm(config.Route, string(data), work_num)
			}
		}
	} else {
		log.Println("now not support mq and method ", config.Mq, config.Method)
	}
}

//通过fastcgi发送数据给fpm
func requestFpm(route route, data string, work_num <-chan int) {
	defer func() {
		consumer_wg.Done()
		<- work_num
	}()

	reqParams := "data="+data

	uri := ""
	if "Index" != route.Module {
		uri = "/"+route.Module
	}

	env := make(map[string]string)
	env["REQUEST_METHOD"] = "POST"
	env["SCRIPT_FILENAME"] = getParentDirectory(getCurrentDirectory())+"/public/consumer.php"
	env["REQUEST_URI"] = uri+"/"+route.Controller+"/"+route.Action
	env["SERVER_SOFTWARE"] = "go / fastcgiclient "
	env["REMOTE_ADDR"] = "127.0.0.1"
	env["SERVER_PROTOCOL"] = "HTTP/1.1"
	env["QUERY_STRING"] = reqParams
	env["PATH_INFO"] = env["REQUEST_URI"]

	fcgi, err := fcgiclient.New("127.0.0.1", 9000)
	if err != nil {
		log.Println("fastcgi connect err ", err)
		return

	}
	defer fcgi.Close()

	resp, err := fcgi.Get(env)
	if err != nil {
		log.Println("fastcgi response err ", err)
		return
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("err:", err)
		return
	}
	log.Println(string(content))

	return
}

//连接redis
func connectRedis(host string, port string, auth string, db int) (redis.Conn, error) {
	conn, err := redis.Dial("tcp", host+":"+port)
	if err != nil {
		return nil, err
	}

	if auth != "" {
		_, err = conn.Do("AUTH", auth)
		if err != nil {
			return nil, err
		}
	}
	if db != 0 {
		_, err = conn.Do("SELECT", db)
		if err != nil {
			return nil, err
		}
	}

	return conn, err
}

func subStr(s string, pos, length int) string {
	runes := []rune(s)
	l := pos + length
	if l > len(runes) {
		l = len(runes)
	}
	return string(runes[pos:l])
}

func getParentDirectory(dirctory string) string {
	return subStr(dirctory, 0, strings.LastIndex(dirctory, "/"))
}

func getCurrentDirectory() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	return strings.Replace(dir, "\\", "/", -1)
}