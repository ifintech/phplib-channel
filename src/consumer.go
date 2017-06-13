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
)

type config struct {
	mq	 string
	method	 string
	host     string
	port     int
	db       int
	auth     string
	max_work int
	topic	 string
	route	 route
}

type route struct {
	modules string
	controller string
	action string
}

var configs map[string]config
var wg sync.WaitGroup
var sign_chans map[string]chan int

func main() {
	//使用上多核
	runtime.GOMAXPROCS(runtime.NumCPU())

	//注册信号
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	//读取配置
	config_bytes, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal("config file not exist",err.Error())
	}
	configs = make(map[string]config)
	err = json.Unmarshal(config_bytes, &configs)
	if err != nil {
		log.Fatal("json decoding faild", err.Error())
	}

	//针对每个topic，开协程处理
	for name,value := range configs {
		go consumer(name, value)
	}

	//主进程不退出，直到收到信号退出，同时通知协程停止获取数据，处理完积压数据
	for {
		select {
			case <-sigs:
				for _,consumer_chan := range sign_chans {
					consumer_chan <- 0
				}
				break
		}
	}

	//等待协程完成,退出
	wg.Wait()
	log.Panicln("exit");
}

//获取消息队列数据，协程发送请求给fastcgi，获得返回结果
func consumer(name string, config config){
	//从队列获取的数据暂存在channel里,最大100，满了就阻塞
	queue := make(chan string, 100)
	//设置最大的请求并发量
	work_num := make(chan int, config.max_work)
	//获取数据
	if(config.mq == "redis" && config.method == "pop"){
		//链接mq
		conn,_ := connectRedis(config.host, config.port, config.auth, config.db)
		//获取数据
		for {
			select {
				case <- sign_chans[name]:
					break
				default:
					for {
						data, _ := redis.Bytes(conn.Do("POP", config.topic))
						queue <- string(data)
					}
			}
		}
	}else{

	}
	//并发调用fastcgi，控制并发量
	for{
		select {
		case <- queue:
			data := <-queue
			go send(config.route, data, work_num)
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
	env["REQUEST_URI"] = "/"+route.modules+"/"+route.controller+"/"+route.action
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
