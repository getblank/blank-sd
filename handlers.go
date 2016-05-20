package main

import (
	"errors"
	"strings"

	"github.com/getblank/blank-sr/config"
	"github.com/getblank/blank-sr/registry"
	"github.com/getblank/blank-sr/sessionstore"
	"github.com/getblank/wango"
)

func registryHandler(c *wango.Conn, uri string, args ...interface{}) (interface{}, error) {
	services := registry.GetAll()
	return services, nil
}

func configHandler(c *wango.Conn, uri string, args ...interface{}) (interface{}, error) {
	conf := config.GetAllStoreObjectsFromDb()
	return conf, nil
}

func registerHandler(c *wango.Conn, uri string, args ...interface{}) (interface{}, error) {
	if args == nil {
		return nil, ErrInvalidArguments
	}

	mes, ok := args[0].(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid register message")
	}

	_type, ok := mes["type"]
	if !ok {
		return nil, errors.New("Invalid register message. No type")
	}
	typ, ok := _type.(string)
	if !ok || typ == "" {
		return nil, errors.New("Invalid register message. No type")
	}
	remoteAddr := "ws://" + strings.Split(c.RemoteAddr(), ":")[0]
	if remoteAddr == "ws://[" {
		remoteAddr = "ws://127.0.0.1"
	}
	var port string
	if _port, ok := mes["port"]; ok {
		port, ok = _port.(string)
	}
	registry.Register(typ, remoteAddr, port, c.ID())

	return nil, nil
}

// args: uri string, event interface{}, subscribers array of connIDs
// This data will be transfered sent as event on "events" topic
func publishHandler(c *wango.Conn, _uri string, args ...interface{}) (interface{}, error) {
	uri, ok := args[0].(string)
	if !ok {
		return nil, ErrInvalidArguments
	}
	_, ok = args[2].([]interface{})
	if !ok {
		return nil, ErrInvalidArguments
	}
	message := map[string]interface{}{
		"event":       args[1],
		"subscribers": args[2],
	}
	wamp.Publish(uri, message)

	return nil, nil
}

func newSessionHandler(c *wango.Conn, uri string, args ...interface{}) (interface{}, error) {
	if args == nil {
		return nil, ErrInvalidArguments
	}
	userId, ok := args[0].(string)
	if !ok {
		return nil, ErrInvalidArguments
	}

	return sessionstore.New(userId).APIKey, nil
}

func checkSessionByApiKeyHandler(c *wango.Conn, uri string, args ...interface{}) (interface{}, error) {
	if args == nil {
		return nil, ErrInvalidArguments
	}
	apiKey, ok := args[0].(string)
	if !ok {
		return nil, ErrInvalidArguments
	}

	s, err := sessionstore.GetByApiKey(apiKey)
	if err != nil {
		return nil, err
	}
	return s.GetUserID(), nil
}

func getSessionByUserIDHandler(c *wango.Conn, uri string, args ...interface{}) (interface{}, error) {
	if args == nil {
		return nil, ErrInvalidArguments
	}
	userID, ok := args[0].(string)
	if !ok {
		return nil, ErrInvalidArguments
	}

	return sessionstore.GetByUserID(userID)
}

func deleteSessionHandler(c *wango.Conn, uri string, args ...interface{}) (interface{}, error) {
	if args == nil {
		return nil, ErrInvalidArguments
	}
	apiKey, ok := args[0].(string)
	if !ok {
		return nil, ErrInvalidArguments
	}

	s, err := sessionstore.GetByApiKey(apiKey)
	if err != nil {
		return nil, err
	}
	s.Delete()

	return nil, nil
}

// args must have 4 members
// apiKey, connID, uri string, extra interface{}
func sessionSubscribedHandler(c *wango.Conn, _uri string, args ...interface{}) (interface{}, error) {
	if len(args) < 4 {
		return nil, ErrInvalidArguments
	}
	apiKey, ok := args[0].(string)
	if !ok {
		return nil, ErrInvalidArguments
	}
	s, err := sessionstore.GetByApiKey(apiKey)
	if err != nil {
		return nil, err
	}
	connID, ok := args[1].(string)
	if !ok {
		return nil, ErrInvalidArguments
	}
	uri, ok := args[2].(string)
	if !ok {
		return nil, ErrInvalidArguments
	}
	extra := args[3]
	s.AddSubscription(connID, uri, extra)

	publishSession(s)

	return nil, nil
}

// args must have 3 members
// apiKey, connID, uri string
func sessionUnsubscribedHandler(c *wango.Conn, _uri string, args ...interface{}) (interface{}, error) {
	if len(args) < 4 {
		return nil, ErrInvalidArguments
	}
	apiKey, ok := args[0].(string)
	if !ok {
		return nil, ErrInvalidArguments
	}
	s, err := sessionstore.GetByApiKey(apiKey)
	if err != nil {
		return nil, err
	}
	connID, ok := args[1].(string)
	if !ok {
		return nil, ErrInvalidArguments
	}
	uri, ok := args[2].(string)
	if !ok {
		return nil, ErrInvalidArguments
	}
	s.DeleteSubscription(connID, uri)
	publishSession(s)

	return nil, nil
}

// args must have 2 members
// apiKey, connID string
func sessionDeleteConnectionHandler(c *wango.Conn, uri string, args ...interface{}) (interface{}, error) {
	if len(args) < 4 {
		return nil, ErrInvalidArguments
	}
	apiKey, ok := args[0].(string)
	if !ok {
		return nil, ErrInvalidArguments
	}
	s, err := sessionstore.GetByApiKey(apiKey)
	if err != nil {
		return nil, err
	}
	connID, ok := args[1].(string)
	if !ok {
		return nil, ErrInvalidArguments
	}
	s.DeleteConnection(connID)
	publishSession(s)

	return nil, nil
}
