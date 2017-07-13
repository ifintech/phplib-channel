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
	"os/exec"
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

const MQ_METHOD_POP = "pop"
const MQ_METHOD_SUB = "sub"

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

	//检查fastcgi服务是否开启
	cmd := "netstat -an | grep 9000"
	err = exec.Command("bash","-c", cmd).Run()
	if err != nil {
		log.Fatal("fastcgi服务未启动")
		return
	}

	//每个topic协程需要接受主进程停止信号
	var consumer_sig_chans = make(map[string](chan os.Signal))

	for name, config := range configs {
		consumer_sig_chans[name] = make(chan os.Signal, 1)

		go consume(name, config, consumer_sig_chans[name])
	}

	Loop:
	//主进程不退出，直到收到信号退出，同时通知协程停止获取数据，处理完积压数据
	for {
		select {
		case sig := <-sigs:
			log.Println("master receive receive signal " + sig.String())

			for _, sig_chan := range consumer_sig_chans {
				sig_chan <- sig
			}

			break Loop
		}
	}

	//等待协程完成,退出
	consumer_wg.Wait()
	log.Println("safe exit");
}

//获取消息队列数据，以channel方式返回
func consume(name string, config config, sig_chan <-chan os.Signal){
	consumer_wg.Add(1)
	defer consumer_wg.Done()

	var task_wg sync.WaitGroup

	//设置最大的请求并发量
	work_num := make(chan int, config.Max_work)

	log.Println(config.Mq, config.Method)

	if(config.Mq == "redis" && MQ_METHOD_POP == config.Method){
		//链接redis todo db&auth
		conn, err := redis.Dial("tcp", config.Host+":"+fmt.Sprint(config.Port))
		if err != nil {
			log.Fatal("mq "+name+" connect err",err.Error())
		}
		defer conn.Close()

		Loop:
		for {
			select {
			case sig := <-sig_chan:
				log.Println("consumer " + name + " receive signal " + sig.String())
				task_wg.Wait()
				log.Println("break consumer: " + name)

				break Loop
			default:
				task_wg.Add(1)
				work_num <- 1

				data, err := redis.Bytes(conn.Do("lpop", config.Topic))
				if err != nil {
					task_wg.Done()
					<-work_num

					log.Println(name + " pop err: ", err.Error())
					time.Sleep(time.Second * 1)
					continue
				}

				log.Println(name + " get data: ", string(data))

				go func() {
					defer func() {
						task_wg.Done()
						<- work_num
					}()

					requestFpm(config.Route, string(data))
				}()
			}
		}
	} else if (config.Mq == "redis" && MQ_METHOD_SUB == config.Method) {
		conn, err := redis.Dial("tcp", config.Host+":"+fmt.Sprint(config.Port))
		if err != nil {
			log.Fatal("mq "+name+" connect err",err.Error())
		}
		defer conn.Close()

		psc := redis.PubSubConn{conn}
		psc.Subscribe(config.Topic)

		var sub_wg sync.WaitGroup
		sub_wg.Add(1)

		go func() {
			defer sub_wg.Done()

			LoopSub:
			for{
				switch msg := psc.Receive().(type) {
				case redis.Message:
					task_wg.Add(1)
					work_num <- 1

					log.Println(name + " get sub data: ", string(msg.Data))

					go func() {
						defer func() {
							task_wg.Done()
							<- work_num
						}()

						requestFpm(config.Route, string(msg.Data))
					}()
				case redis.Subscription:
					log.Printf(name + " subscription info: %s %s %d\n", msg.Kind, msg.Channel, msg.Count)
					if msg.Count == 0 {
						return
					}
				case error:
					log.Println(name + " sub err: ", msg.Error())

					break LoopSub
				}
			}
		}()

		LoopSubSig:
		for {
			select {
			case sig := <-sig_chan:
				psc.Unsubscribe(config.Topic)

				sub_wg.Wait()

				log.Println("consumer " + name + " receive signal " + sig.String())
				task_wg.Wait()
				log.Println("break consumer: " + name)

				break LoopSubSig
			}
		}
	} else {
		log.Println("now not support mq and method ", config.Mq, config.Method)
	}
}

//通过fastcgi发送数据给fpm
func requestFpm(route route, data string) {
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
		log.Println("fastcgi connect err: ", err)
		return

	}
	defer fcgi.Close()

	resp, err := fcgi.Get(env)
	if err != nil {
		log.Println("fastcgi response err: ", err)
		return
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("err:", err)
		return
	}
	log.Println("response msg: " , string(content))

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