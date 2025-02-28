package api

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/raft"
	"gorm.io/gorm"
	"log"
	"raft_learning/service/controller"
	"raft_learning/service/mapper"
	"raft_learning/service/service"
)

func RegisterRouter(r *gin.Engine, database *mapper.CacheManager, db *gorm.DB, redis *redis.Client, ctx context.Context, raft *raft.Raft) {
	sm := &mapper.StudentMapper{Database: database, DB: db, Redis: redis, Ctx: ctx}
	ss := &service.StudentService{StudentMapper: sm, Raft: raft}
	sc := &controller.StudentController{StudentService: ss}

	// 预加热数据
	err := sm.PreloadCache()
	if err != nil {
		log.Println(err)
	}

	// 注册student路由组
	student := r.Group("/student")
	{
		student.POST("/save", sc.SetStudent)
		student.GET("/search", sc.GetStudent)
		student.PUT("/update", sc.UpdateStudent)
		student.DELETE("/delete", sc.DeleteStudent)
	}
}
