package main

import (
	"os"
	"sync"
	"log"
	"time"
	"fcgiclient"
	"io/ioutil"
	"queue"
	"mq"
	"config"
	"os/exec"
)

type Worker struct {
	name       string
	config     config.Config
	worker_num chan int
	sig_chan   chan os.Signal
	task_wg    sync.WaitGroup
}

func newWorker(name string, config config.Config, sig_chan chan os.Signal) *Worker {
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
	if (!queue.IsValidType(worker.config.Mq)) {
		log.Println("consumer ", worker.name, " not support queue type ", worker.config.Mq)

		return
	}
	log.Println("consumer ", worker.name, " start pop")

	defer queue.RemoveInstance(worker.config.Mq, worker.name)

	for {
		select {
		case sig := <-worker.sig_chan:
			log.Println("consumer " + worker.name + " receive signal " + sig.String())

			worker.task_wg.Wait()
			log.Println("break consumer: " + worker.name)

			return
		default:
			q, err := queue.GetInstance(worker.name, worker.config)
			if err != nil {
				log.Println(worker.name + " get queue instance err: ", err.Error())
				time.Sleep(time.Second * 1)
				continue
			}
			if (!isFpmOn()) {
				log.Println(worker.name, "fpm off")
				time.Sleep(time.Second * 1)
				continue
			}

			worker.task_wg.Add(1)
			worker.worker_num <- 1

			data, err := q.Pop()
			if (nil != err) {
				log.Println(worker.name + " pop err: ", err.Error())

				worker.task_wg.Done()
				<-worker.worker_num

				//断线后清除实例, 再次循环时重新获取新实例
				queue.RemoveInstance(worker.config.Mq, worker.name)
				continue
			}
			if ("" == data) {
				worker.task_wg.Done()
				<-worker.worker_num

				time.Sleep(time.Second * 1)
				continue
			}

			log.Println(worker.name + " pop data: ", data)

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

//订阅消息队列
func (worker *Worker) doSub() {
	if (!mq.IsValidType(worker.config.Mq)) {
		log.Println("consumer ", worker.name, " not support mq type: ", worker.config.Mq)

		return
	}
	log.Println("consumer ", worker.name, " start sub")

	defer mq.RemoveInstance(worker.config.Mq, worker.name)

	var is_run bool
	var sub_wg sync.WaitGroup
	var q mq.Mq

	sub_wg.Add(1)

	go func() {
		is_run = true

		var err error
		defer sub_wg.Done()

		for{
			if (!is_run) {
				return
			}

			q, err = mq.GetInstance(worker.name, worker.config)
			if (nil != err) {
				log.Println(worker.name + " get mq instance err: ", err.Error())
				time.Sleep(time.Second * 1)
				continue
			}
			if (!isFpmOn()) {
				log.Println(worker.name, "fpm off")
				time.Sleep(time.Second * 1)
				continue
			}

			data, err := q.Sub()
			if (nil != err) {
				//断线后清除实例, 再次循环时重新获取新实例
				mq.RemoveInstance(worker.config.Mq, worker.name)

				log.Println(worker.name + " sub err: ", err.Error())
				continue
			}
			if ("" != data) {
				worker.task_wg.Add(1)
				worker.worker_num <- 1

				log.Println(worker.name + " get sub data: ", data)

				go func() {
					defer func() {
						worker.task_wg.Done()
						<- worker.worker_num
					}()

					requestFpm(worker.config.Route, data)
				}()
			}
		}
	}()

	for {
		select {
		case sig := <-worker.sig_chan:
			log.Println("consumer " + worker.name + " receive signal: " + sig.String())

			if (nil != q) {
				q.UnSub()
			}
			is_run = false

			sub_wg.Wait()
			worker.task_wg.Wait()

			log.Println("break consumer: " + worker.name)

			return
		}
	}
}

func isFpmOn() bool {
	cmd := "netstat -anpl | grep 9000"
	err := exec.Command("bash", "-c", cmd).Run()

	if err == nil {
		return true
	} else {
		return false
	}
}

//通过fastcgi发送数据给fpm
func requestFpm(route config.Route, data string) {
	reqParams := "data="+data

	uri := ""
	if "Index" != route.Module {
		uri = "/"+route.Module
	}

	env := make(map[string]string)
	env["REQUEST_METHOD"] = "POST"
	env["SCRIPT_FILENAME"] = config.GetParentDirectory(config.GetCurrentDirectory())+"/public/consumer.php"
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