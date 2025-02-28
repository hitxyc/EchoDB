package main

import (
	"fmt"
	"github.com/go-redis/redis/v8"
	"golang.org/x/net/context"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"raft_learning/config"
	"raft_learning/raftNode"
	"raft_learning/service/api"
	"raft_learning/service/mapper"
	"time"
)

var db *gorm.DB
var rdb *redis.Client
var ctx = context.Background()

func main() {
	// 构建数据库
	database := &mapper.CacheManager{}
	database.Init()
	// 启动第一个节点并作为Bootstrap节点
	node := &raftNode.RaftNode{}
	// 初始设置，将其设置为第一节点, 其中localID要设为类似"nodeName: localhost:8080"的形式以便重定向找到leader
	node.InitOptions(true, "node1:localhost:7070", "localhost:5000", "localhost:5000")
	go func() {
		err := node.NewRaftNode(&mapper.StudentMapper{DB: db, Redis: rdb, Ctx: ctx, Database: database})
		if err != nil {
			panic(err)
		}
	}()
	time.Sleep(1 * time.Second)
	// 注册api服务
	port := ":7070"
	err := api.RouterInit(database, db, rdb, ctx, node.Raft, port)
	if err != nil {
		panic(err)
	}
}

func init() {
	config_, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}
	// 配置数据库
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config_.Database.Username, config_.Database.Password, config_.Database.Host, config_.Database.Port, config_.Database.DatabaseName)
	db_, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic(err)
	}
	db = db_
	// 初始化 Redis 客户端
	rdb_ := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password
		DB:       0,  // default DB
	})
	rdb = rdb_
}
