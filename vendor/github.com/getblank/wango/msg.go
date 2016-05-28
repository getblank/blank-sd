package wango

import "encoding/json"

type wampMsg struct {
	ID   interface{}
	URI  string
	Args []interface{}
}

type Error struct {
	Desc string `json:"desc"`
}

func createMessage(args ...interface{}) ([]byte, error) {
	return json.Marshal(args)
}

func createHeartbeatEvent(counter int) ([]byte, error) {
	return createMessage(msgHeartbeat, counter)
}

func createHeartbeatTxtEvent(counter int) ([]byte, error) {
	return createMessage(msgIntTypes[msgHeartbeat], counter)
}

func createWelcomeMessage(id string) ([]byte, error) {
	return createMessage(msgWelcome, id, 1, identity)
}

func createError(err interface{}) Error {
	var text string
	switch err.(type) {
	case error:
		text = err.(error).Error()
	case string:
		text = err.(string)
	}
	return Error{"error#" + text}
}
