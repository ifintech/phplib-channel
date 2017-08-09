package main

import (
	"config"
	"fcgiclient"
	"io/ioutil"
	"log"
	"mq"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

const FPM_HOST = "127.0.0.1"
const FPM_PORT = 9000

const TYPE_REDIS = "redis"

var types = []string{
	TYPE_REDIS,
}

type Worker struct {
	name       string
	config     config.Config
	worker_num chan int
	sig_chan   chan os.Signal
	task_wg    sync.WaitGroup
}

func newWorker(name string, config config.Config) *Worker {
	var task_wg sync.WaitGroup
	//设置最大的请求并发量
	worker_num := make(chan int, config.Max_work)
	sig_chan := make(chan os.Signal, 1)

	return &Worker{
		name:       name,
		config:     config,
		worker_num: worker_num,
		sig_chan:   sig_chan,
		task_wg:    task_wg,
	}
}

func isValidType(queue_type string) bool {
	for _, item := range types {
		if item == queue_type {
			return true
		}
	}
	return false
}

//主动拉取消息队列
func (worker *Worker) doPop() {
	if !isValidType(worker.config.Mq) {
		log.Println("consumer", worker.name, "not support mq:", worker.config.Mq)
		return
	}

	log.Println("consumer", worker.name, "start", worker.config.Method, worker.config.Topic)
	for {
		select {
		case sig := <-worker.sig_chan:
			log.Println("consumer", worker.name, "receive signal", sig.String())
			worker.task_wg.Wait()
			log.Println("break consumer", worker.name)

			return
		default:
			if !isFpmOn() {
				log.Println(worker.name, "fpm off")
				time.Sleep(time.Second * 1)
				continue
			}

			worker.task_wg.Add(1)
			worker.worker_num <- 1

			var data string
			var err error

			if worker.config.Mq == "redis" {
				data, err = mq.RedisPop(worker.name, worker.config)
			}

			if err != nil {
				log.Println(worker.name, "pop err:", err.Error())

				worker.task_wg.Done()
				<-worker.worker_num

				continue
			}

			if "" == data {
				worker.task_wg.Done()
				<-worker.worker_num

				time.Sleep(time.Second * 1)
				continue
			}

			log.Println(worker.name, "pop data:", data)

			go func() {
				defer func() {
					worker.task_wg.Done()
					<-worker.worker_num
				}()

				requestFpm(worker.config, data)
			}()
		}
	}
}

func isFpmOn() bool {
	conn, err := net.Dial("tcp", FPM_HOST+":"+strconv.Itoa(FPM_PORT))
	if err == nil {
		conn.Close()
		return true
	} else {
		return false
	}
}

//通过fastcgi发送数据给fpm
func requestFpm(conf config.Config, data string) {
	reqParams := "data=" + data

	env := make(map[string]string)
	env["REQUEST_METHOD"] = "POST"
	env["SCRIPT_FILENAME"] = "/data1/htdocs/" + config.GetAppName() + "/public/consumer.php"
	env["REQUEST_URI"] = conf.Uri
	env["SERVER_SOFTWARE"] = "go / fastcgiclient "
	env["REMOTE_ADDR"] = "127.0.0.1"
	env["SERVER_PROTOCOL"] = "HTTP/1.1"
	env["QUERY_STRING"] = reqParams
	env["PATH_INFO"] = env["REQUEST_URI"]

	fcgi, err := fcgiclient.New(FPM_HOST, FPM_PORT)
	if err != nil {
		log.Println("fastcgi connect err: ", err)
		return
	}
	defer fcgi.Close()

	resp, err := fcgi.Get(env)
	if err != nil {
		log.Println("fastcgi response err:", err)
		return
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("err:", err)
		return
	}
	log.Println("response msg:", string(content))

	return
}
