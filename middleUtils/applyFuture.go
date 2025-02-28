package middleUtils

import (
	"fmt"
	"github.com/hashicorp/raft"
	"log"
	"sync"
	"time"
)

/*
	这是一个来连接service的raft.Apply函数和fsm的apply函数而设计的中间件
	起因是发现上述的两个apply函数之间存在黑箱(即不知道这两个函数之间具体干了什么)
	导致mapper返回的error无法传输到service层, 只能在日志里显示出现了错误
	为解决这一问题, 而设计了下面基于select的连接两个apply函数的ApplyFuture对象
	事务逻辑是mapper返回的error(没有则返回nil), 会利用SetError函数放入err通道, 并关闭err和done通道, 在select语句中检测到done通道关闭而退出循环
	在关闭done通道后, 主函数的select语句会结束, 返回raft.Apply的future.Error
	在新建一个ApplyFuture节点时会顺便清理掉已经done的节点, 防止map中数据过多
	在设计这个中间件时想到如果用二者利用tcp进行交流或许也许也是个不错的解决方法
**/

// ApplyFuture 是一个用于等待日志应用结果的结构体
type ApplyFuture struct {
	done chan struct{} // 用于同步的通道
	err  chan error    // 错误信息
}

type ApplyList struct {
	applyFutureMap map[int]ApplyFuture
	length         int
	lock           sync.Locker
}

var applyList ApplyList

func init() {
	applyList = ApplyList{}
	applyList.applyFutureMap = make(map[int]ApplyFuture)
	applyList.lock = new(sync.Mutex)
}

// NewApplyFuture 新建一个applyFuture节点, 同时清理已用完的节点
func NewApplyFuture() ApplyFuture {
	applyList.lock.Lock()
	defer applyList.lock.Unlock()
	// 创建applyFuture节点
	applyFuture := &ApplyFuture{}
	applyFuture.done = make(chan struct{})
	applyFuture.err = make(chan error, 1)
	applyList.applyFutureMap[applyList.length] = *applyFuture
	applyList.length++
	// 清理已使用完的通道
	func() {
		for index, applyFuture := range applyList.applyFutureMap {
			select {
			case <-applyFuture.done: // 如果 done通道和err通道 都已关闭
				if _, ok := <-applyFuture.err; !ok {
					// 从map中删除已关闭的 ApplyFuture
					delete(applyList.applyFutureMap, index)
				}
			default:
				// 如果 done 通道和err通道 还没有关闭，跳过该元素
			}
		}
	}()
	return *applyFuture
}

func GetApplyFuture() *ApplyFuture {
	future := applyList.applyFutureMap[applyList.length-1]
	return &future
}

// Apply 用于从 Raft 状态机中提交日志条目并获取结果
func (f *ApplyFuture) Apply(entry []byte, raft *raft.Raft) error {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()
	if entry == nil {
		f.err <- fmt.Errorf("invalid log entry: nil")
		close(f.done)
		return nil
	}
	future := raft.Apply(entry, 10*time.Second)
	go func() {
		select {
		case <-time.After(300 * time.Second):
			f.err <- fmt.Errorf("timed out")
			close(f.err)
			close(f.done)
		case _, ok := <-f.done:
			if !ok {
				log.Println("middleUtil's log: done channel has been closed")
			}
		}
	}()
	select {
	case _, ok := <-f.done:
		if !ok {
			log.Println("middleUtil's log: finished apply")
		}
	}
	return future.Error()
}

// SetError 用于设置错误信息，并关闭通道
func (f *ApplyFuture) SetError(err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()
	f.err <- err
	close(f.err) // 确保错误通道只关闭一次
	log.Println("middleUtil's log: the err channel is closed")
	close(f.done) // 确保 done 通道只关闭一次
}

func (f *ApplyFuture) GetError() error {
	var err error
	select {
	case err = <-f.err:
		return err
	case <-time.After(300 * time.Second):
		log.Println("middleUtil's log: get err timeout")
		return fmt.Errorf("middleUtil's log: get err timeout")
	}
}
