package zk_distribution_lock

import (
	"context"
	"errors"
	"github.com/samuel/go-zookeeper/zk"
	"log"
	"path"
	"sort"
	"strings"
	"time"
)

//DistributeLock defines interface for all distribution locks
type DistributeLock interface {
	Lock() error
	Release() error
}

const (
	//BaseLockNode holds all distribute locks
	DefaultLockNodeBase = "/distribute-locks"
)

//DistributeLocker implements a distribute lock with zookeeper
type DistributeLocker struct {
	driver *zk.Conn
	name string
	lockNodeBase string
	lockName string
	timeout time.Duration
}

//NewLocker returns a locker with a distribute lock created in zk
func NewLocker(conn *zk.Conn, opt ...func(locker *DistributeLocker)) (*DistributeLocker, error) {
	locker := &DistributeLocker{}
	locker.driver = conn
	for _, fn := range opt {
		fn(locker)
	}
	if locker.lockNodeBase == "" {
		locker.lockNodeBase = DefaultLockNodeBase
	}
	return locker, nil
}

//WithBasePath sets default lock node path base dir
func WithBasePath(path string) func(locker *DistributeLocker) {
	return func(locker *DistributeLocker) {
		locker.lockNodeBase = strings.TrimRight(path, "/")
		splitPath := strings.Split(strings.Trim(path, "/"), "/")
		pathBuilder := strings.Builder{}
		pathBuilder.WriteString("/")
		for _, p := range splitPath {
			pathBuilder.WriteString(p)
			err := locker.createIfNotExits(pathBuilder.String())
			if err != nil {
				panic(err)
			}
			pathBuilder.WriteString("/")
		}
	}
}

func WithTimeout(n time.Duration) func(locker *DistributeLocker) {
	return func(locker *DistributeLocker) {
		locker.timeout = n
	}
}

func (locker *DistributeLocker) createIfNotExits(path string) error {
	flag, _, err := locker.driver.Exists(path)
	if err != nil {
		return err
	}
	if flag {
		return nil
	}
	_,err = locker.driver.Create(
		path,
		nil,
		0,
		zk.WorldACL(zk.PermAll))
	if err != nil {
		log.Println("could not create base node, reason:"+err.Error())
		return err
	}
	return nil
}

//buildLock tries to build a new lock with given lock name
func (locker *DistributeLocker) createLock(lockName string) error {
	// create a lock node
	node,err := locker.driver.Create(
		locker.lockNodeBase+"/"+lockName,
		nil,
		zk.FlagSequence|zk.FlagEphemeral,
		zk.WorldACL(zk.PermAll),
	)
	if err != nil {
		return err
	}
	locker.name = node
	return nil
}

func (locker *DistributeLocker) Lock(lockName string) error {
	locker.lockName = lockName
	err := locker.createLock(lockName)
	if err != nil {
		return err
	}
	locks,err := locker.getLocks()
	if err != nil {
		return err
	}
	sort.Strings(locks)
	_, currentNodeName := path.Split(locker.name)
	if locks[0] == currentNodeName {
		return nil
	}
	idx := searchStrings(locks, currentNodeName)
	if result := <-locker.watchExit(locks[idx - 1]); result == true {
		return nil
	}
	return errors.New("wait lock timeout")
}

func (locker *DistributeLocker) watchExit(nodeName string) <-chan bool  {
	log.Println(locker.name, "is waiting for", locker.lockNodeBase+"/"+nodeName)
	ch := make(chan bool, 1)
	var ctx context.Context
	var cancel context.CancelFunc
	if locker.timeout > 0 {
		ctx,cancel = context.WithTimeout(context.Background(), locker.timeout)
	} else {
		ctx,cancel = context.WithCancel(context.Background())
	}
	defer cancel()
	result, _, eCh, err := locker.driver.ExistsW(locker.lockNodeBase+"/"+nodeName)
	if err != nil {
		log.Println("node watch failed",err)
		ch <- false
		return ch
	}
	if !result {
		ch <- true
		return ch
	}
	for {
		select {
		case event := <- eCh:
			if event.Type == zk.EventNodeDeleted {
				ch <- true
				return ch
			}
		case <-ctx.Done():
			log.Println("wait lock timeout")
			ch <- false
			return ch
		}
	}
}

func searchStrings(search []string, needle string) int {
	length := len(search)
	for i:=0; i < length; i ++ {
		if search[i] == needle {
			return i
		}
	}
	return -1
}

//getLocks returns list of current lock's series
func (locker *DistributeLocker) getLocks() ([]string, error) {
	nodes, _, err := locker.driver.Children(locker.lockNodeBase)
	if err != nil {
		return nil, err
	}
	results := make([]string, 0)
	for _, node := range nodes {
		if !strings.HasPrefix(node, locker.lockName) {
			continue
		}
		results = append(results, node)
	}
	return results,nil
}

//Release
func (locker *DistributeLocker) Release() error {
	return locker.release()
}

//release does real zk node remove job
func (locker *DistributeLocker) release() error {
	log.Println("releasing lock", locker.name)
	flag, _, err := locker.driver.Exists(locker.name)
	if err != nil {
		return err
	}
	if !flag {
		return nil
	}
	err = locker.driver.Delete(
		locker.name,
		-1,
	)
	if err != nil {
		return err
	}
	return nil
}

func (locker *DistributeLocker) Name() string {
	return locker.name
}

