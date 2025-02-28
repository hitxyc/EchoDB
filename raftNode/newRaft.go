package raftNode

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/hashicorp/raft"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"raft_learning/service/mapper"
	"time"
)

// Options 新增的配置项，用于区分是否是Bootstrap节点
type Options struct {
	bootstrap      bool
	localID        string // Raft的ID
	raftTCPAddress string // Raft 集群内部通信的地址
	joinAddress    string // 加入集群的第一个节点地址
}

type RaftNode struct {
	opts *Options
	Raft *raft.Raft
}

// InitOptions 初始设置
func (raftNode *RaftNode) InitOptions(bootstrap bool, localID string, raftTCPAddress string, joinAddress string) {
	raftNode.opts = &Options{bootstrap: bootstrap, localID: localID, raftTCPAddress: raftTCPAddress, joinAddress: joinAddress}
}

/*
	Config： 节点配置
	FSM： finite state machine，有限状态机
	LogStore： 用来存储raft的日志
	StableStore： 稳定存储，用来存储raft集群的节点信息等
	SnapshotStore: 快照存储，用来存储节点的快照信息
	Transport： raft节点内部的通信通道

*
*/
func (raftNode *RaftNode) NewRaftNode(sm *mapper.StudentMapper) error {
	config := newConfig(raftNode.opts.localID)
	// LogStore、StableStore分别用来存储raft log、节点状态信息, 这里将数据储存在内存中
	logStore := raft.NewInmemStore()
	stableStore := raft.NewInmemStore()
	// 创建快照存储实例，可以使用自定义的快照存储，例如 FileSnapshotStore
	snapshotStore := raft.NewInmemSnapshotStore()

	// 生成 transport 对象用于网络通信
	addr, err := net.ResolveTCPAddr("tcp", raftNode.opts.joinAddress)
	if err != nil {
		return err
	}
	transport, err := raft.NewTCPTransport(raftNode.opts.raftTCPAddress, addr, 3, 10*time.Second, os.Stdout)
	if err != nil {
		return fmt.Errorf("failed to create transport: %v", err)
	}

	// 创建一个 FSM 实例
	fsm := &FSM{sm: sm}

	// 创建Raft实例
	raft_, err := raft.NewRaft(config, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return fmt.Errorf("failed to create Raft node: %v", err)
	}
	raftNode.Raft = raft_

	// 如果是Bootstrap节点，调用 BootstrapCluster
	if raftNode.opts.bootstrap {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}

		// 初始化并让该节点成为 leader
		future := raftNode.Raft.BootstrapCluster(configuration)
		if err = future.Error(); err != nil {
			return fmt.Errorf("failed to bootstrap cluster: %v", err)
		}
		// 为leader开启joinHTTP服务
		err = joinHTTPService(":5050", raftNode.Raft)
		if err != nil {
			return err
		}
	}
	// 如果不是Bootstrap节点, 加入集群
	if !raftNode.opts.bootstrap {
		err = joinByHTTP(raftNode.opts)
		if err != nil {
			return err
		}
	}
	return nil
}

// join joins a node, identified by nodeID and located at addr, to this store.
// The node must be ready to respond to Raft communications at that address.
func join(raft_ *raft.Raft, peerID string, peerAddr string) error {

	configFuture := raft_.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		log.Printf("failed to get raft configuration: %v", err)
		return err
	}
	// 查看是否有重复id或地址的节点
	for _, srv := range configFuture.Configuration().Servers {
		// If a node already exists with either the joining node's ID or address,
		// that node may need to be removed from the config first.
		if srv.ID == raft.ServerID(peerID) || srv.Address == raft.ServerAddress(peerAddr) {
			// However if *both* the ID and the address are the same, then nothing -- not even
			// a join operation -- is needed.
			if srv.Address == raft.ServerAddress(peerAddr) && srv.ID == raft.ServerID(peerID) {
				log.Printf("node %s at %s already member of cluster, ignoring join request", peerID, peerAddr)
				return nil
			}

			future := raft_.RemoveServer(srv.ID, 0, 0)
			if err := future.Error(); err != nil {
				return fmt.Errorf("error removing existing node %s at %s: %s", peerID, peerAddr, err)
			}
		}
	}
	// 利用AddVoter加入集群
	f := raft_.AddVoter(raft.ServerID(peerID), raft.ServerAddress(peerAddr), 0, 0)
	if f.Error() != nil {
		return f.Error()
	}
	log.Printf("node %s at %s joined successfully", peerID, peerAddr)
	return nil
}

// JoinHTTPService 给join开一个HTTP服务以使peer找到集群
func joinHTTPService(joinAddr string, raft_ *raft.Raft) error {
	// 创建一个Gin引擎
	r := gin.Default()
	// 设置一个简单的路由
	r.GET("/join", func(c *gin.Context) {
		peerID := c.Query("peerID")
		peerAddr := c.Query("peerAddr")
		err := join(raft_, peerID, peerAddr)
		if err != nil {
			log.Printf("failed to join raft node: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to join raft node" + err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": "success"})
	})

	// 指定端口并手动启动服务
	var err error
	port := joinAddr
	go func() {
		err = r.Run(":5050")
		if err != nil {
			log.Println("Error starting server:", err)
		}
	}()

	log.Printf("Starting server on %s...\n", port)
	return err
}

// JoinByHTTP 通过网络加入集群
func joinByHTTP(opts *Options) error {
	// 进行http请求
	url := fmt.Sprintf("http://%s/join?peerID=%s&peerAddr=%s", "localhost:5050", opts.localID, opts.raftTCPAddress)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// 读取响应内容
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}

	// 打印响应内容
	log.Println("Response:", string(body))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("join request failed: %s", string(body))
	}
	return nil
}
