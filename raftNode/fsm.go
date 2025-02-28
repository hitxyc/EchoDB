package raftNode

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hashicorp/raft"
	"io"
	"log"
	"raft_learning/middleUtils"
	"raft_learning/service/entity"
	"raft_learning/service/mapper"
)

/*
FSM 接口概述
Raft 的 FSM 接口提供了三种方法：
1.Apply(*Log) interface{}：当日志条目被提交后，Raft 会调用此方法来执行日志中的操作。
对于一个状态机来说，执行这些日志就是对其内部状态进行更新（例如在键值存储中进行数据修改）。
2.Snapshot()：为了支持日志压缩，Raft 提供了快照的功能，允许将状态机的当前状态保存为一个快照，节省存储空间并提高恢复速度。
3.Restore(io.ReadCloser) error：用于从快照恢复状态机的状态。
*/
type FSM struct {
	sm          *mapper.StudentMapper
	applyFuture middleUtils.ApplyFuture
}

type LogEntryData struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// FSMSnapshot 为了兼容Raft的快照和恢复，我们需要定义 FSMSnapshot,
// 首先我们需要定义快照，hashicorp/raft内部定义了快照的interface，需要实现两个方法, persist和release
type FSMSnapshot struct {
	Students []entity.Student `json:"students"`
}

// Persist 来生成快照数据
func (s *FSMSnapshot) Persist(sink raft.SnapshotSink) error {
	// Serialize the snapshot data (Students list) into JSON
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal FSM snapshot: %v", err)
	}

	// Write the data to the SnapshotSink
	_, err = sink.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write snapshot data: %v", err)
	}
	// Indicate that the snapshot has been successfully persisted
	return sink.Close()
}

// Release 快照处理完成后的回调，不需要的话可以实现为空函数。
func (s *FSMSnapshot) Release() {
	// No resources to release in this example, but this is where you'd clean up
	fmt.Println("Snapshot resources released")
}

// Apply 方法实现：将日志条目应用到状态机
func (fsm *FSM) Apply(logEntry *raft.Log) interface{} {
	// 处理panic
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in apply", r)
		}
	}()
	//解码日志条目数据
	e := LogEntryData{}
	if err := json.Unmarshal(logEntry.Data, &e); err != nil {
		log.Println("Failed unmarshalling Raft log entry.")
	}
	// 进行具体事务
	fsm.sm.Database.Lock()
	defer fsm.sm.Database.Unlock()
	future := middleUtils.GetApplyFuture()
	switch e.Key {
	case "add_student":
		err := fsm.addStudent(e)
		if err != nil {
			err2 := fmt.Errorf("failed adding student: %v", err)
			future.SetError(err2)
			log.Printf("fsm's log failed adding student: %v", err2)
		} else {
			future.SetError(nil)
		}
	case "update_student":
		err := fsm.updateStudent(e)
		if err != nil {
			err2 := fmt.Errorf("failed updating student: %v", err)
			future.SetError(err2)
			log.Printf("fsm's log failed updating student: %v", err2)
		} else {
			future.SetError(nil)
		}
	case "delete_student":
		err := fsm.deleteStudent(e)
		if err != nil {
			err2 := fmt.Errorf("failed deleting student: %v", err)
			future.SetError(err2)
			log.Printf("fsm's log failed deleting student: %v", err2)
		} else {
			future.SetError(nil)
		}
	}

	return e
}

func (fsm *FSM) addStudent(e LogEntryData) error {
	// 假设日志条目包含新学生的信息
	student := entity.Student{}
	if err := json.Unmarshal([]byte(e.Value), &student); err != nil {
		log.Println("Failed unmarshalling student data.")
		return err
	}
	// 在此调用mapper的SetStudent方法来更新数据库
	sm := fsm.sm
	err := sm.SetStudent(&student)
	if err != nil {
		// 处理错误
		log.Println(fmt.Sprintf("Failed to set student: %v", err))
		return err
	}
	return nil
}

func (fsm *FSM) updateStudent(e LogEntryData) error {
	information := map[string]interface{}{}
	if err := json.Unmarshal([]byte(e.Value), &information); err != nil {
		log.Println("Failed unmarshalling student data.")
		return err
	}
	id, ok := information["id"].(string)
	if !ok {
		log.Println("Failed getting student_id")
		return errors.New("failed getting student_id")
	}
	// 绑定学生信息
	student := entity.Student{}
	jsonData, err := json.Marshal(information["student"])
	if err != nil {
		log.Println("Failed marshalling student data.")
	}
	if err = json.Unmarshal(jsonData, &student); err != nil {
		log.Println("Failed unmarshalling student data.")
		return err
	}
	// 修改学生信息
	sm := fsm.sm
	err = sm.UpdateStudent(&id, &student)
	if err != nil {
		log.Println(fmt.Sprintf("Failed to update student: %v", err))
		return err
	}
	return nil
}

func (fsm *FSM) deleteStudent(e LogEntryData) error {
	information := map[string]string{}
	if err := json.Unmarshal([]byte(e.Value), &information); err != nil {
		log.Println("Failed unmarshalling student data.")
		return err
	}
	id := information["id"]
	sm := fsm.sm
	err := sm.DeleteStudent(&id)
	if err != nil {
		log.Println(fmt.Sprintf("Failed to delete student: %v", err))
		return err
	}
	return nil
}

// Snapshot 方法实现：获取状态机的快照
func (fsm *FSM) Snapshot() (raft.FSMSnapshot, error) {
	fsm.sm.Database.RLock()
	defer fsm.sm.Database.RUnlock()

	// 创建并返回快照（可以考虑将数据库序列化）
	snapshot := &FSMSnapshot{
		Students: fsm.sm.Database.Data.Students,
	}
	return snapshot, nil
}

// Restore 方法实现：从快照恢复状态机
func (fsm *FSM) Restore(snapshot io.ReadCloser) error {
	// 读取快照数据并恢复状态
	var snapshotData FSMSnapshot
	if err := json.NewDecoder(snapshot).Decode(&snapshotData); err != nil {
		return err
	}
	// 进行具体事务
	fsm.sm.Database.Lock()
	defer fsm.sm.Database.Unlock()
	// 恢复数据
	fsm.sm.Database.Data.Students = snapshotData.Students
	fsm.sm.Database.Data.Length = len(snapshotData.Students)
	for i, student := range snapshotData.Students {
		fsm.sm.Database.Data.StudentMap[student.Id] = i
	}
	return nil
}
