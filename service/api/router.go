package api

import (
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/raft"
	"golang.org/x/net/context"
	"gorm.io/gorm"
	"raft_learning/service/mapper"
)

func RouterInit(database *mapper.CacheManager, db *gorm.DB, rdb *redis.Client, ctx context.Context, raft *raft.Raft, port string) error {
	// 启动Gin路由
	router := gin.Default()
	RegisterRouter(router, database, db, rdb, ctx, raft)
	err := router.Run(port)
	return err
}
