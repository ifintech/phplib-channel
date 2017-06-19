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

func main() {
	//使用上多核
	runtime.GOMAXPROCS(runtime.NumCPU())

	//注册信号
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	//读取配置
	config_bytes, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal("config file not exist err:",err.Error())
	}
	var configs map[string]config
	err = json.Unmarshal(config_bytes, &configs)
	if err != nil {
		log.Fatal("json decoding faild err:", err.Error())
	}

	//todo 检查fastcgi服务是否开启

	//每个topic协程需要接受主进程停止信号
	var sign_consumers map[string]chan int
	//每个topic协程数据暂存区
	var data_consumers map[string]chan string

	for name,value := range configs {
		go getData(name, value, sign_consumers[name], data_consumers[name])
	}

	//发送给fpm处理数据
	for name,value := range configs {
		go consumer(name, value, data_consumers[name])
	}

	//主进程不退出，直到收到信号退出，同时通知协程停止获取数据，处理完积压数据
	for {
		select {
		case <-sigs:
			for _,consumer_chan := range sign_consumers {
				consumer_chan <- 0
			}
			break
		}
	}

	//等待协程完成,退出
	wg.Wait()
	log.Panicln("safe exit");
}

//获取消息队列数据，以channel方式返回
func getData(name string, config config, sign_consumer chan int, data_consumer chan string){
	wg.Add(1)
	defer wg.Done()
	//获取数据
	log.Println(config.Mq, config.Method)
	if(config.Mq == "redis" && config.Method == "pop"){
		//链接redis todo db&auth
		conn, err := redis.Dial("tcp", config.Host+":"+string(config.Port))
		if err != nil {
			log.Fatal("mq "+name+" connect err",err.Error())
		}
		defer conn.Close()

		//获取数据
		for {
			select {
			case <- sign_consumer:
				break
			default:
				for {
					data, err := redis.Bytes(conn.Do("POP", config.Topic))
					if err != nil {
						log.Println("pop err",err.Error())
						time.Sleep(1)
						continue
					}
					if data == nil {
						log.Println("pop nil data")
						time.Sleep(1)
						continue
					}
					data_consumer <- string(data)
				}
			}
		}
	} else {
		log.Println("now not support mq and method ", config.Mq, config.Method)
	}
}

func consumer(name string, config config, data_consumer chan string){
	wg.Add(1)
	defer wg.Done()
	//设置最大的请求并发量
	work_num := make(chan int, config.Max_work)
	//并发调用fastcgi，控制并发量
	for{
		select {
		case <- data_consumer:
			data := <-data_consumer
			go send(config.Route, data, work_num)
		}
	}
}

//通过fastcgi发送数据给fpm
func send(route route, data string, work_num chan int) {
	work_num <- 1

	reqParams := "data="+data

	env := make(map[string]string)
	env["REQUEST_METHOD"] = "POST"
	env["SCRIPT_FILENAME"] = getParentDirectory(getCurrentDirectory())+"/public/consumer.php"
	env["REQUEST_URI"] = "/"+route.Module+"/"+route.Controller+"/"+route.Action
	env["SERVER_SOFTWARE"] = "go / fastcgiclient "
	env["REMOTE_ADDR"] = "127.0.0.1"
	env["SERVER_PROTOCOL"] = "HTTP/1.1"
	env["QUERY_STRING"] = reqParams

	fcgi, err := fcgiclient.New("127.0.0.1", 9000)
	if err != nil {
		log.Println("fastcgi connect err ", err)
		return

	}
	defer func() {
		<- work_num
		fcgi.Close()
	}()

	resp, err := fcgi.Get(env)
	if err != nil {
		log.Println("fastcgi response err ", err)
		return
	}
	defer func() {
		<- work_num
	}()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("err:", err)
		return
	}
	defer func() {
		<- work_num
	}()

	log.Println(string(content))
	<- work_num
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

func substr(s string, pos, length int) string {
	runes := []rune(s)
	l := pos + length
	if l > len(runes) {
		l = len(runes)
	}
	return string(runes[pos:l])
}

func getParentDirectory(dirctory string) string {
	return substr(dirctory, 0, strings.LastIndex(dirctory, "/"))
}

func getCurrentDirectory() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	return strings.Replace(dir, "\\", "/", -1)
}
