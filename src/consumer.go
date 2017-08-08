package main

import (
	"flag"
	"github.com/erikdubbelboer/gspt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"fmt"
)

const METHOD_POP = "pop"

const PID_FILE_PATH = "/var/run/"
const CONFIG_FILE_PATH = "/data1/htdocs/%s/bin/channel.json"

func main() {
	App_name = flag.String("n", "", "config file path")
	flag.Parse()

	//使用上多核
	runtime.GOMAXPROCS(runtime.NumCPU())

	//pid
	setProcTitle()
	recycleLastPid()
	savePid()

	//注册信号
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	var workers = make(map[string](*Worker))

	config_path := fmt.Sprintf(CONFIG_FILE_PATH, App_name);
	configs := LoadConfig(config_path)

	for name, conf := range configs {
		sig_chan := make(chan os.Signal, 1)
		workers[name] = newWorker(name, conf, sig_chan)
		wg.Add(1)
		go func(worker *Worker) {
			defer wg.Done()
			consume(worker)
		}(workers[name])
	}

Loop:
	//主进程阻塞直到收到信号退出，同时通知协程停止获取数据，处理完积压数据
	for {
		select {
		case sig := <-sigs:
			log.Println("master receive receive signal", sig.String())

			for _, worker := range workers {
				worker.sig_chan <- sig
			}

			break Loop
		}
	}

	//等待协程完成,退出
	wg.Wait()
	log.Println("safe exit")
}

//消费消息队列
func consume(worker *Worker) {
	if METHOD_POP == worker.config.Method {
		worker.doPop()
	} else {
		log.Println("consumer", worker.name, "not support method:", worker.config.Method)
	}
}

func setProcTitle() {
	gspt.SetProcTitle("php-fpm: pool consumer channel")
}

func recycleLastPid() {
	last_pid_str, _ := ioutil.ReadFile(getPidFile())
	last_pid, _ := strconv.Atoi(string(last_pid_str))

	if last_pid > 0 {
		err := syscall.Kill(last_pid, syscall.SIGTERM)

		if nil == err {
			log.Println("kill last pid", last_pid)
		} else {
			log.Println("kill last pid", last_pid, "err:", err)
		}
	}
}

func savePid() {
	pid := os.Getpid()
	pid_file := getPidFile()
	err := ioutil.WriteFile(pid_file, []byte(strconv.Itoa(pid)), 0644)
	if nil != err {
		log.Println("err saving pid", pid, "to", pid_file, "err:", err)
	}
}

func getPidFile() string {
	return PID_FILE_PATH + "php-fpm.consumer.pid"
}
