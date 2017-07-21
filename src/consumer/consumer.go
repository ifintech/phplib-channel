package main

import (
	"config"
	"github.com/erikdubbelboer/gspt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

const METHOD_POP = "pop"
const METHOD_SUB = "sub"

const PID_FILE_PATH = "/var/run/"

func main() {
	//使用上多核
	runtime.GOMAXPROCS(runtime.NumCPU())

	setProcTitle()
	recycleLastPid()
	savePid()

	//注册信号
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	var worker_sigs = make(map[string](chan os.Signal))

	configs := config.LoadConfig()
	for name, conf := range configs {
		worker_sigs[name] = make(chan os.Signal, 1)
		wg.Add(1)

		go func(name string, conf config.Config, sig_chan <-chan os.Signal) {
			defer wg.Done()

			consume(name, conf, sig_chan)
		}(name, conf, worker_sigs[name])
	}

Loop:
	//主进程不退出，直到收到信号退出，同时通知协程停止获取数据，处理完积压数据
	for {
		select {
		case sig := <-sigs:
			log.Println("master receive receive signal " + sig.String())

			for _, sig_chan := range worker_sigs {
				sig_chan <- sig
			}

			break Loop
		}
	}

	//等待协程完成,退出
	wg.Wait()
	log.Println("safe exit")
}

//消费消息队列
func consume(name string, config config.Config, sig_chan <-chan os.Signal) {
	var task_wg sync.WaitGroup
	//设置最大的请求并发量
	worker_num := make(chan int, config.Max_work)

	worker := Worker{
		name:       name,
		config:     config,
		worker_num: worker_num,
		sig_chan:   sig_chan,
		task_wg:    task_wg,
	}
	if METHOD_POP == config.Method {
		worker.doPop()
	} else if METHOD_SUB == config.Method {
		worker.doSub()
	} else {
		log.Println("consumer ", name, " not support method: ", config.Method)
	}
}

func setProcTitle() {
	gspt.SetProcTitle("CONSUMER_" + getAppName())
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
	return PID_FILE_PATH + "consumer_" + getAppName() + ".pid"
}

func getAppName() string {
	app_path := config.GetParentDirectory(config.GetCurrentDirectory())
	dir := strings.Split(app_path, "/")
	app_name := dir[len(dir)-1]

	return app_name
}