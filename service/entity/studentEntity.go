package entity

type Student struct {
	Id      string `json:"id" form:"id" gorm:"column:id"`
	Name    string `json:"name" form:"name" gorm:"column:name"`
	Age     int    `json:"age" form:"age" gorm:"column:age"`
	Deleted bool   `json:"deleted" form:"deleted" gorm:"column:deleted"`
}

// TableName 返回表名
func (*Student) TableName() string {
	return "student"
}

// OmitEmpty 修改信息时忽略空值
func (stu *Student) OmitEmpty(old *Student) {
	if stu.Id == "" {
		stu.Id = old.Id
	}
	if stu.Name == "" {
		stu.Name = old.Name
	}
	if stu.Age == old.Age {
		stu.Age = old.Age
	}
}
