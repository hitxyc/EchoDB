package entity

type ResultEntity struct {
	Message string      `json:"message"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}
