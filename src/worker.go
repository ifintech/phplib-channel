package main

import (
	"fcgiclient"
	"mq"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

const FPM_HOST = "127.0.0.1"
const FPM_PORT = 9000

type Worker struct {
	name       string
	config     Config
	worker_num chan int
	sig_chan   chan os.Signal
	task_wg    sync.WaitGroup
}

func newWorker(name string, config Config, sig_chan chan os.Signal) *Worker {
	var task_wg sync.WaitGroup
	//设置最大的请求并发量
	worker_num := make(chan int, config.Max_work)

	return &Worker{
		name:       name,
		config:     config,
		worker_num: worker_num,
		sig_chan:   sig_chan,
		task_wg:    task_wg,
	}
}

//主动拉取消息队列
func (worker *Worker) doPop() {
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

			if worker.config.Mq == "redis" {
				data, err := mq.RedisPop()
			}

			if err != nil {
				log.Println(worker.name, "pop err:", err.Error())

				worker.task_wg.Done()
				<-worker.worker_num

				continue
			}

			if data == nil {
				worker.task_wg.Done()
				<-worker.worker_num

				time.Sleep(time.Second * 1)
				continue
			}

			log.Println(worker.name+" pop data: ", data)

			go func() {
				defer func() {
					worker.task_wg.Done()
					<-worker.worker_num
				}()

				requestFpm(worker.config.Route, data)
			}()
		}
	}
}

func isFpmOn() bool {
	cmd := "netstat -anpl | grep " + fmt.Sprint(FPM_PORT)
	err := exec.Command("bash", "-c", cmd).Run()

	if err == nil {
		return true
	} else {
		return false
	}
}

//通过fastcgi发送数据给fpm
func requestFpm(config Config, data string) {
	reqParams := "data=" + data

	env := make(map[string]string)
	env["REQUEST_METHOD"] = "POST"
	env["SCRIPT_FILENAME"] = "/data1/htdocs/" + App_name + "/public/consumer.php"
	env["REQUEST_URI"] = config.Uri
	env["SERVER_SOFTWARE"] = "go / fastcgiclient "
	env["REMOTE_ADDR"] = "127.0.0.1"
	env["SERVER_PROTOCOL"] = "HTTP/1.1"
	env["QUERY_STRING"] = reqParams
	env["PATH_INFO"] = env["REQUEST_URI"]

	fcgi, err := fcgiclient.New(FPM_HOST, FPM_PORT)
	defer fcgi.Close()

	if err != nil {
		log.Println("fastcgi connect err: ", err)
		return

	}

	resp, err := fcgi.Get(env)
	defer resp.Body.Close()

	if err != nil {
		log.Println("fastcgi response err:", err)
		return
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("err:", err)
		return
	}
	log.Println("response msg:", string(content))

	return
}
