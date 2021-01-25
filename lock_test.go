package zk_distribution_lock

import (
	"context"
	"fmt"
	"github.com/samuel/go-zookeeper/zk"
	"github.com/stretchr/testify/assert"
	"log"
	"sync"
	"testing"
	"time"
)

func TestGetInstance(t *testing.T) {
	wg :=sync.WaitGroup{}
	conn, _, err := zk.Connect([]string{"127.0.0.1"}, time.Minute, zk.WithLogInfo(false))
	if err != nil {
		panic(err)
	}
	for i:=0; i < 5000; i ++ {
		wg.Add(1)
		go func() {
			locker, err := NewLocker(conn, WithBasePath("/test/"))
			if err != nil {
				panic(err)
			}
			log.Println("try to get lock")
			err = locker.Lock("lock")
			if err != nil {
				log.Println(err)
				t.Fail()
			}
			log.Println("thread get lock successful", locker.Name())
			defer func() {
				err = locker.Release()
			}()
			assert.Nil(t, err, "can not create locker", err)
			assert.IsType(t, &DistributeLocker{}, locker, "can not initialize locker")
			wg.Done()
		}()
	}
	wg.Wait()
	conn.Close()
}
