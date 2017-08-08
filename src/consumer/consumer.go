package main

import (
	"config"
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
)

const PID_FILE_PATH = "/var/run/"

func main() {
	ptr_config_file := flag.String("config_file", "", "json type config which contains consumer info")
	flag.Parse()

	config_file := *ptr_config_file
	if "" == config_file {
		log.Fatal("config file should be provided")
	}
	configs := config.LoadConfig(config_file)

	//使用上多核
	runtime.GOMAXPROCS(runtime.NumCPU())

	setProcTitle()
	recycleLastPid()
	savePid()

	//注册信号
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	var workers = make(map[string](*Worker))

	for name, conf := range configs {
		workers[name] = newWorker(name, conf)
		wg.Add(1)
		go func(worker *Worker) {
			defer wg.Done()

			worker.do()
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

func setProcTitle() {
	gspt.SetProcTitle("CONSUMER_" + config.GetAppName())
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
	return PID_FILE_PATH + "consumer_" + config.GetAppName() + ".pid"
}
