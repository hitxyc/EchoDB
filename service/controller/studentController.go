package controller

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"raft_learning/service/entity"
	"raft_learning/service/service"
	"strings"
)

type StudentController struct {
	StudentService *service.StudentService
}

func (sc *StudentController) SetStudent(c *gin.Context) {
	var student entity.Student
	err := c.ShouldBind(&student)
	if err != nil {
		c.JSON(http.StatusBadRequest, entity.ResultEntity{Message: err.Error(), Success: false})
		return
	}
	log.Println(student)
	// 调用StudentService的SetStudent方法进行Raft操作
	err = sc.StudentService.SetStudent(&student)
	if err != nil {
		message := strings.Split(err.Error(), ": ")
		log.Println(err.Error())
		// 如果message前缀是 "the leader is at", 则转发给leader
		if len(message) > 1 && message[0] == "the leader is at" {
			addr := fmt.Sprintf("http://%s/student/save", message[1])
			log.Printf("redirect to leader at %s\n", addr)
			// 转发表单信息
			dataList := []string{"id", "name", "age"}
			redirectToLeader(c, addr, "POST", &dataList)
		} else {
			c.JSON(http.StatusBadRequest, entity.ResultEntity{Message: err.Error(), Success: false})
		}
		return
	}
	c.JSON(http.StatusOK, entity.ResultEntity{Message: fmt.Sprintf("The Student %+v is saved", student),
		Success: true})
}

func (sc *StudentController) GetStudent(c *gin.Context) {
	index := c.Query("id")
	stu, err := sc.StudentService.GetStudent(&index)
	if err != nil {
		c.JSON(http.StatusBadRequest, entity.ResultEntity{Message: err.Error(), Success: false})
		return
	}
	c.JSON(http.StatusOK, entity.ResultEntity{Message: "The Student is found", Success: true, Data: stu})
}

func (sc *StudentController) UpdateStudent(c *gin.Context) {
	id := c.Query("id")
	var student entity.Student
	err := c.ShouldBind(&student)
	if err != nil {
		c.JSON(http.StatusBadRequest, entity.ResultEntity{Message: err.Error(), Success: false})
	}
	// 调用StudentService的UpdateStudent方法进行Raft操作
	err = sc.StudentService.UpdateStudent(&id, &student)
	if err != nil {
		message := strings.Split(err.Error(), ": ")
		log.Println("studentController's log:" + err.Error())
		// 如果message前缀是 "the leader is at", 则转发给leader
		if len(message) > 1 && message[0] == "the leader is at" {
			addr := fmt.Sprintf("http://%s/student/update?id=%s", message[1], id)
			log.Printf("redirect to leader at %s\n", addr)
			// 转发信息
			dataList := []string{"id", "name", "age"}
			redirectToLeader(c, addr, "PUT", &dataList)
		} else {
			c.JSON(http.StatusBadRequest, entity.ResultEntity{Message: err.Error(), Success: false})
		}
		return
	}
	c.JSON(http.StatusOK, entity.ResultEntity{Message: fmt.Sprintf("The Student %+v is updated", id), Success: true})
}

func (sc *StudentController) DeleteStudent(c *gin.Context) {
	id := c.Query("id")
	// 调用StudentService的DeleteStudent方法进行Raft操作
	err := sc.StudentService.DeleteStudent(&id)
	if err != nil {
		message := strings.Split(err.Error(), ": ")
		log.Println(err.Error())
		// 如果message前缀是 "the leader is at", 则转发给leader
		if len(message) > 1 && message[0] == "the leader is at" {
			addr := fmt.Sprintf("http://%s/student/delete?id=%s", message[1], id)
			log.Printf("redirect to leader at %s\n", addr)
			redirectToLeader(c, addr, "DELETE", nil)
		} else {
			c.JSON(http.StatusBadRequest, entity.ResultEntity{Message: err.Error(), Success: false})
		}
		return
	}
	c.JSON(http.StatusOK, entity.ResultEntity{Message: fmt.Sprintf("The Student %+v is deleted", id), Success: true})
}

func redirectToLeader(c *gin.Context, addr string, method string, formDate *[]string) {
	var client *http.Client
	var req *http.Request
	var err error
	switch method {
	case "POST":
		client, req, err = redirectPOST(c, addr, formDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, entity.ResultEntity{Message: err.Error(), Success: false})
			return
		}
	case "PUT":
		client, req, err = redirectPUT(c, addr, formDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, entity.ResultEntity{Message: err.Error(), Success: false})
			return
		}
	case "DELETE":
		client, req, err = redirectDELETE(c, addr, formDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, entity.ResultEntity{Message: err.Error(), Success: false})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, entity.ResultEntity{Message: fmt.Sprintf("Invalid method %s", method), Success: false})
	}
	// 发送请求
	log.Printf("sending request to leader at %s\n", addr)
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, entity.ResultEntity{Message: fmt.Sprintf("%v,the leader is at %s", err.Error(), addr), Success: false})
		return
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	// 根据 leader 响应返回结果
	if resp.StatusCode == http.StatusOK {
		c.JSON(http.StatusOK, entity.ResultEntity{Message: "Request forwarded to leader successfully, leader is at " + addr, Success: true, Data: string(body)})
	} else { // 创建 ResultEntity 实例
		var result entity.ResultEntity
		// 使用 json.Unmarshal 解码 JSON 字符串
		err := json.Unmarshal(body, &result)
		if err != nil {
			fmt.Println("Error unmarshalling:", err)
			return
		}
		c.JSON(http.StatusInternalServerError,
			entity.ResultEntity{Message: fmt.Sprintf("redirect to leader successfully but failed to finish the request, the error is: %s, leader is at %s", result.Message, addr)})
	}
}

// 转发POST请求
func redirectPOST(c *gin.Context, addr string, dataList *[]string) (*http.Client, *http.Request, error) {
	var err error
	var formData url.Values
	// 判断是否存在表单数据
	if dataList != nil {
		// 获得form单数据
		var data []interface{}
		for _, dataNeeded := range *dataList {
			data = append(data, c.PostForm(dataNeeded))
		}
		// 构造新的表单数据
		formData = url.Values{}
		for index, dataNeeded := range *dataList {
			formData.Set(dataNeeded, fmt.Sprint(data[index]))
		}
		log.Println(formData)
	}
	// 使用 HTTP 客户端将请求转发到 leader
	client := &http.Client{}
	req := &http.Request{}
	if formData != nil {
		req, err = http.NewRequest("POST", addr, strings.NewReader(formData.Encode()))
	} else {
		req, err = http.NewRequest("POST", addr, nil)
	}
	if err != nil {
		return nil, nil, err
	}
	// 设置请求头 Content-Type 为 application/x-www-form-urlencoded
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return client, req, nil
}

func redirectPUT(c *gin.Context, addr string, dataList *[]string) (*http.Client, *http.Request, error) {
	var err error
	var formData url.Values
	// 判断是否存在表单数据
	if dataList != nil {
		// 获得form单数据
		var data []interface{}
		for _, dataNeeded := range *dataList {
			data = append(data, c.PostForm(dataNeeded))
		}
		// 构造新的表单数据
		formData = url.Values{}
		for index, dataNeeded := range *dataList {
			formData.Set(dataNeeded, fmt.Sprint(data[index]))
		}
		log.Println(formData)
	}
	// 使用 HTTP 客户端将请求转发到 leader
	client := &http.Client{}
	req := &http.Request{}
	if formData != nil {
		req, err = http.NewRequest("PUT", addr, strings.NewReader(formData.Encode()))
	} else {
		req, err = http.NewRequest("PUT", addr, nil)
	}
	if err != nil {
		return nil, nil, err
	}
	// 设置请求头 Content-Type 为 application/x-www-form-urlencoded
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return client, req, nil
}

func redirectDELETE(c *gin.Context, addr string, dataList *[]string) (*http.Client, *http.Request, error) {
	var err error
	var formData url.Values
	// 判断是否存在表单数据
	if dataList != nil {
		// 获得form单数据
		var data []interface{}
		for _, dataNeeded := range *dataList {
			data = append(data, c.PostForm(dataNeeded))
		}
		// 构造新的表单数据
		formData = url.Values{}
		for index, dataNeeded := range *dataList {
			formData.Set(dataNeeded, fmt.Sprint(data[index]))
		}
		log.Println(formData)
	}
	// 使用 HTTP 客户端将请求转发到 leader
	client := &http.Client{}
	req := &http.Request{}
	if formData != nil {
		log.Println("new request, formData:", formData)
		req, err = http.NewRequest("DELETE", addr, strings.NewReader(formData.Encode()))
	} else {
		log.Println("new request, formData is nil")
		req, err = http.NewRequest("DELETE", addr, nil)
	}
	if err != nil {
		return nil, nil, err
	}
	// 设置请求头 Content-Type 为 application/x-www-form-urlencoded
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return client, req, nil
}
