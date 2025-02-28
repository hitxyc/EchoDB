package mapper

import (
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"golang.org/x/net/context"
	"gorm.io/gorm"
	"raft_learning/service/entity"
	"time"
)

type StudentMapper struct {
	Database *CacheManager
	DB       *gorm.DB
	Redis    *redis.Client
	Ctx      context.Context
}

// SetStudent 创建学生
func (sm *StudentMapper) SetStudent(student *entity.Student) error {
	db := sm.Database
	// 查看学生是否已经存在
	if _, ok := db.Data.StudentMap[student.Id]; ok {
		return errors.New("student already exists")
	}
	_, err := sm.getStudentFromCache(student.Id, true)
	if err != nil {
		return err
	}
	// 如果database、redis和mysql都没找到, 将数据添加入database
	db.Data.StudentMap[student.Id] = db.Data.Length
	db.Data.Students = append(db.Data.Students, *student)
	db.Data.Length++
	return nil
}

// GetStudent 通过id查询学生信息
func (sm *StudentMapper) GetStudent(id *string) (*entity.Student, error) {
	db := sm.Database
	index, ok := db.Data.StudentMap[*id]
	// 如果database未找到, 再到redis或mysql中寻找
	if !ok {
		stu, err := sm.getStudentFromCache(*id, false)
		if err != nil {
			return nil, fmt.Errorf("student does not exist or has other error: %s", err)
		}
		return stu, nil
	}
	return &db.Data.Students[index], nil
}

// UpdateStudent 修改学生信息
func (sm *StudentMapper) UpdateStudent(id *string, student *entity.Student) error {
	db := sm.Database
	oldStudentIndex, ok := db.Data.StudentMap[*id]
	// 如果database未找到, 再到redis或mysql中寻找
	if !ok {
		stu, err := sm.getStudentFromCache(*id, false)
		if err != nil {
			return fmt.Errorf("student does not exist or has other error: %s", err)
		}
		// 忽略空值
		student.OmitEmpty(stu)
		// 判断学生id是否发生改变
		if student.Id == stu.Id {
			sm.Redis.Set(sm.Ctx, student.Id, student, 24*time.Hour)
		} else {
			// 发生改变
			sm.Redis.Del(sm.Ctx, *id)
			sm.Redis.Set(sm.Ctx, student.Id, student, 24*time.Hour)
		}
		if err = sm.DB.Model(&entity.Student{}).Where("id=?", *id).Updates(*student).Error; err != nil {
			return err
		}
		return nil
	}
	oldStudent := db.Data.Students[oldStudentIndex]
	// 忽略空值
	student.OmitEmpty(&oldStudent)
	// 判断学生id是否发生改变
	if student.Id == oldStudent.Id {
		db.Data.Students[oldStudentIndex] = *student
	} else {
		// 发生改变, 修改map表
		db.Data.Students[oldStudentIndex] = *student
		delete(db.Data.StudentMap, oldStudent.Id)
		db.Data.StudentMap[student.Id] = oldStudentIndex
	}
	return nil
}

// DeleteStudent 删除学生信息
func (sm *StudentMapper) DeleteStudent(id *string) error {
	db := sm.Database
	index, ok := db.Data.StudentMap[*id]
	// 如果database未找到, 再到redis或mysql中寻找
	if !ok {
		stu, err := sm.getStudentFromCache(*id, false)
		if err != nil {
			return fmt.Errorf("student does not exist or has other error: %s", err)
		}
		stu.Deleted = true
		sm.Redis.Del(sm.Ctx, *id)
		if err = sm.DB.Model(&entity.Student{}).Where("id=?", *id).Update("deleted", true).Error; err != nil {
			return err
		}
		return nil
	}
	// 进行假删除
	stu := db.Data.Students[index]
	stu.Deleted = true
	delete(db.Data.StudentMap, *id)
	return nil
}
