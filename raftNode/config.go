package raftNode

import (
	"github.com/hashicorp/raft"
	"time"
)

func newConfig(localID string) *raft.Config {
	// config是节点的配置信息，我们直接使用raft默认的配置
	config := raft.DefaultConfig()
	// 设置 Raft 节点的唯一标识符。每个 Raft 节点在集群中都有一个唯一的 ID，用来区分不同的节点。
	{
		config.LocalID = raft.ServerID(localID)
	}
	// 设置 Raft 心跳超时的时间间隔。
	{
		//config.HeartbeatTimeout = 150 * time.Millisecond
		//config.LeaderLeaseTimeout = 100 * time.Millisecond
	}
	// 设置选举超时的时间间隔。Raft 节点会在此时间内等待来自领导者的心跳信号。如果未收到心跳，则节点会开始新的领导者选举。
	{
		//config.ElectionTimeout = 300 * time.Millisecond
	}
	// 设置提交日志条目到状态机的超时。这个超时用来确保在 Raft 中只有当日志条目被大多数节点复制之后，才会提交到状态机。
	{
		// config.CommitTimeout = 100 * time.Millisecond
	}
	// 设置日志级别，控制 Raft 实例的日志输出。通常有 info、debug 和 error 等级别，用来帮助开发者调试和监控 Raft 集群的状态。
	{
		// config.LogLevel = "info"
	}
	// 为保证数据的一致性，只能通过leader写入数据，因此需要及时了解leader的变更信息，在Raft的配置中有一个变量NotifyCh chan<- bool，
	// 当Raft变为leader时会将true写入该chan，通过读取该chan来判断本节点是否是leader。
	{
		leaderNotifyCh := make(chan bool, 10)
		config.NotifyCh = leaderNotifyCh
	}
	// 而快照生成和保存的触发条件除了应用程序主动触发外，还可以在Config里面设置SnapshotInterval和SnapshotThreshold，
	// 前者指每间隔多久生成一次快照，后者指每commit多少log entry后生成一次快照。需要两个条件同时满足才会生成和保存一次快照。
	{
		config.SnapshotInterval = 20 * time.Second
		config.SnapshotThreshold = 5
	}

	return config
}
