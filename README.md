# zk-distribution-lock

#### 介绍
golang 基于 zk 的分布式锁实现方案

#### 软件架构

- 创建一个zk目录 lock
- 线程A想获取锁就在 lock 目录下创建临时顺序节点；
- 获取 lock 目录下所有的子节点，然后获取比自己小的兄弟节点，如果不存在，则说明当前线程顺序号最小，获得锁；
- 线程B获取所有节点，判断自己不是最小节点，设置监听比自己次小的节点；
- 线程A处理完，删除自己的节点，线程B监听到变更事件，判断自己是不是最小的节点，如果是则获得锁。



#### 安装教程
```shell
go get github.com/qunqiang/zk-distribution-lock
```

#### 使用说明

```go
package demo

import (
	lock "github.com/qunqiang/zk-distribution-lock"
	"github.com/samuel/go-zookeeper/zk"
	"time"
	"log"
)

//demo shows a demo function use specific lock name
func demo() {
	conn,_,err := zk.Connect([]string{"127.0.0.1"}, time.Minute,zk.WithLogInfo(false))
	if err != nil {
		panic(err)
    }
	locker, err := lock.NewLocker(conn, lock.WithBasePath("/test/"))
	if err != nil {
		panic(err)
	}
	log.Println("try to get lock")
	err = locker.Lock("lock")
	if err != nil {
		log.Println(err)
		panic(err)
	}
	// do your codes
	log.Println("thread get lock successful", locker.Name())
	defer func() {
		err = locker.Release()
	}()
}
```


#### 参与贡献

1.  Fork 本仓库
2.  新建 Feat_xxx 分支
3.  提交代码
4.  新建 Pull Request

