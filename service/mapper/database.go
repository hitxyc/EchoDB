package mapper

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
	entity "raft_learning/service/entity"
	"sync"
	"time"
)

type Database struct {
	StudentMap map[string]int
	Students   []entity.Student
	Length     int
}

type CacheManager struct {
	Data Database
	sync.RWMutex
}

func (db *CacheManager) Init() {
	var __db__ = Database{
		StudentMap: make(map[string]int),
		Students:   make([]entity.Student, 0),
		Length:     0,
	}

	db.Data = __db__
}

// PreloadCache 预热缓存，加载一些热点数据
func (sm *StudentMapper) PreloadCache() error {
	// 1. 从 MySQL 中加载一些热点数据, 只加载前10条数据
	var students []entity.Student
	if err := sm.DB.Limit(10).Model(&entity.Student{}).Find(&students).Error; err != nil {
		return err
	}
	// 2. 将数据存入 Redis 中
	for _, student := range students {
		// 使用 student.ID 作为缓存的 key
		key := student.Id
		sm.Redis.Set(sm.Ctx, key, student, 24*time.Hour)
		log.Printf("PreloadCache: store in redis %v:%v", key, student)
	}
	return nil
}

// 懒加载数据，如果 Redis 中没有，才从 MySQL 中查询
func (sm *StudentMapper) getStudentFromCache(id string, lookForExist bool) (*entity.Student, error) {
	// 检查 Redis 中是否有该数据
	key := id
	value, err := sm.Redis.Get(sm.Ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		// 如果redis中没有, 则从mysql中寻找
		var student entity.Student
		if err = sm.DB.Model(entity.Student{}).Where("id = ?", id).First(&student).Error; err != nil {
			// 仅查看该学生是否存在, 在此处发现redis和mysql都没找到, 返回err=nil
			if lookForExist {
				return nil, nil
			}
			return nil, err
		}
		// 如果学生已被删除, 返回错误
		if student.Deleted == true {
			return nil, errors.New("the student has been deleted")
		}
		// 将数据存入 Redis
		sm.Redis.Set(sm.Ctx, key, student, 24*time.Hour)
		// 仅查看该学生是否存在
		if lookForExist {
			if student.Deleted == true {
				return nil, nil
			}
			return nil, errors.New("student already exists")
		} else {
			return &student, nil
		}
	} else if err != nil {
		return nil, fmt.Errorf("get student from cache: %w", err)
	}
	// Redis 中有数据
	if lookForExist {
		// 仅查看该学生是否存在
		return nil, errors.New("student already exists")
	}
	jsonData, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var student entity.Student
	if err = json.Unmarshal(jsonData, &student); err != nil {
		return nil, err
	}
	return &student, nil
}
