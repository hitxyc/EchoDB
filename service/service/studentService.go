package service

import (
	"errors"
	"fmt"
	"github.com/goccy/go-json"
	"github.com/hashicorp/raft"
	"raft_learning/middleUtils"
	"raft_learning/raftNode"
	"raft_learning/service/entity"
	"raft_learning/service/mapper"
	"strings"
)

type StudentService struct {
	StudentMapper *mapper.StudentMapper
	Raft          *raft.Raft
}

// 判断是否是leader
func (ss *StudentService) judgeLeader() error {
	// 判断该节点的状态, 如果非leader则无法启动该服务
	if ss.Raft.State() != raft.Leader {
		_, leaderID := ss.Raft.LeaderWithID()
		// 获得leader服务的地址, 根据冒号分割，取第二部分
		parts := strings.Split(string(leaderID), ":")
		if !(len(parts) > 1) {
			// 如果格式不符合，返回错误
			return fmt.Errorf("this node is not leader and leaderID is not standerded: %s", string(leaderID))
		}
		return fmt.Errorf("the leader is at: %s:%s", parts[1], parts[2])
	}
	return nil
}

// 提交日志条目到Raft集群
func (ss *StudentService) apply(eventBytes []byte) error {
	future := middleUtils.NewApplyFuture()
	// 第一个error是raft.Apply返回的future.Error
	err := future.Apply(eventBytes, ss.Raft)
	if err != nil {
		return err
	}
	// 第二个error是在fsm的apply函数返回的error
	err = future.GetError()
	return err
}

// SetStudent 创建学生
func (ss *StudentService) SetStudent(student *entity.Student) error {
	err := ss.judgeLeader()
	if err != nil {
		return err
	}
	// 判断学生信息是否有缺失
	if student == nil {
		return errors.New("student is nil")
	}
	if student.Id == "" {
		return errors.New("student's id is necessary")
	}
	if student.Name == "" {
		return errors.New("student's name is necessary")
	}
	// 转化学生信息为日志条目
	jsonData, err := json.Marshal(student)
	if err != nil {
		return err
	}
	event := raftNode.LogEntryData{Key: "add_student", Value: string(jsonData)}
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}
	// 提交日志条目到Raft集群
	return ss.apply(eventBytes)
}

// GetStudent 通过id查询学生信息
func (ss *StudentService) GetStudent(id *string) (*entity.Student, error) {
	if id == nil || *id == "" {
		return nil, errors.New("student id is necessary")
	}
	return ss.StudentMapper.GetStudent(id)
}

// UpdateStudent 修改学生信息
func (ss *StudentService) UpdateStudent(id *string, student *entity.Student) error {
	// 判断是否为leader
	err := ss.judgeLeader()
	if err != nil {
		return err
	}
	// 判断id是否为空
	if id == nil || *id == "" {
		return errors.New("student id is necessary")
	}
	if student == nil {
		return errors.New("student is nil")
	}
	// 转化信息为日志条目
	information := map[string]interface{}{"id": *id, "student": *student}
	jsonData, err := json.Marshal(information)
	if err != nil {
		return err
	}
	event := raftNode.LogEntryData{Key: "update_student", Value: string(jsonData)}
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}
	// 提交日志条目到Raft集群
	return ss.apply(eventBytes)
}

// DeleteStudent 删除学生信息
func (ss *StudentService) DeleteStudent(id *string) error {
	// 判断是否为leader
	err := ss.judgeLeader()
	if err != nil {
		return err
	}
	// 判断id是否为空
	if id == nil || *id == "" {
		return errors.New("student id is necessary")
	}
	// 转化信息为日志条目
	jsonData, err := json.Marshal(map[string]string{"id": *id})
	if err != nil {
		return err
	}
	event := raftNode.LogEntryData{Key: "delete_student", Value: string(jsonData)}
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}
	// 提交日志条目到Raft集群
	return ss.apply(eventBytes)
}
