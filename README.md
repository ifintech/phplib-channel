# 简介

phplib-channel是一个基于go语言的协程(goroutine)实现的消息队列并行处理服务。主进程用以监控和管理一个或多个监听或轮询消息队列的协程，当任务来临的时候，子协程将任务通过fastcgi协议发送给php-fpm，由fpm派发给其子进程，通过yaf分发给具体的控制器，按照web流程走完全部流程。

## 原由

这样做的好处是：
  * fpm稳定性良好，几乎不存在内存泄露等情况
  * fpm自带进程管理功能，可以根据配置动态调整子进程数量
  * 任务获取和fastcig请求的逻辑可以使用go语言协程的模式完成，与任务的处理逻辑解耦合，使得整个流程的处理模式与web模式进行统一，更加规范
  * 通过单进程+协程的方式实现类似master-worker模型，相比传统的fork多个子进程模式更轻量、更高效、更便于管理

## 目标

快速、高效、稳定地处理消息队列，并通过模拟web请求的方式完成任务

## 使用及示范

1. 部署任务节点

```bash
wget -O /usr/local/bin/consumer https://github.com/ifintech/phplib-channel/raw/master/bin/consumer
chmod +x /usr/local/bin/consumer
```

2. 添加配置文件  

假设业务代码根目录为/data1/htdocs/demo, 在/data1/htdocs/demo/bin/中添加消息队列配置文件channel.json  
配置文件格式示范
```json
{
  "consumer1": {
    "mq": "redis",
    "method":"pop",
    "max_work": 4,
    "topic": "test1",
    "uri": "Consumer/Demo/doSth",
    "dsn": {
      "host": "127.0.0.1",
      "port": 6379,
      "db": 0,
      "auth": "password"
    }
  }
}
```
3. 启动服务
```bash
nohup /usr/local/bin/consumer -f /data1/htdocs/demo/bin/channel.json >> /var/log/consumer.log 2>&1 &
```

## 适用场景

1. 消息队列轮询
2. 消息订阅

## 注意事项

* 配置文件中的进程最大数量(max_work)需要根据fpm配置文件中的进程数合理分配

## todo list

* 提升性能，fastcgi keepalive