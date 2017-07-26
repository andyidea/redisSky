package backend

import (
	"fmt"

	"reflect"

	"github.com/garyburd/redigo/redis"
	gosocketio "github.com/graarh/golang-socketio"
)

// AddKey save to redis
func AddKey(conn *gosocketio.Channel, data interface{}) {
	if c, _redisValue, ok := checkRedisValue(conn, data); ok {
		defer c.Close()
		switch _redisValue.T {
		case "string":
			val, ok := (_redisValue.Val).(string)
			if !ok {
				sendCmdError(conn, "val should be string")
				return
			}
			cmd := "SET " + _redisValue.Key + " " + val
			sendCmd(conn, cmd)
			r, err := c.Do("SET", _redisValue.Key, val)
			if err != nil {
				sendCmdError(conn, err.Error())
				return
			}
			sendCmdReceive(conn, r)
		case "list", "set":
			val, ok := (_redisValue.Val).([]string)
			if !ok {
				sendCmdError(conn, "val should be array of string")
				return
			}
			var method string
			if _redisValue.T == "list" {
				method = "LPUSH"
			} else {
				method = "SADD"
			}
			slice := make([]interface{}, 0, 10)
			slice = append(slice, _redisValue.Key, val)
			cmd := fmt.Sprintf("%s %v", method, slice)
			sendCmd(conn, cmd)
			r, err := c.Do(method, slice...)
			if err != nil {
				sendCmdError(conn, err.Error())
				return
			}
			sendCmdReceive(conn, r)
		case "hash":
			hset(conn, c, _redisValue)
		case "zset":
			zadd(conn, c, _redisValue)
		default:
			sendCmdError(conn, "type is not correct")
			return
		}

		conn.Emit("ReloadKeys", _redisValue.Key)
	}
}

// zadd
func zadd(conn *gosocketio.Channel, c redis.Conn, _redisValue redisValue) bool {
	vals, ok := (_redisValue.Val).(map[string]interface{})
	if !ok {
		sendCmdError(conn, "val should be map of string -> int64 or string -> string")
		return false
	}
	var cmd string
	for k, v := range vals {
		kind := reflect.ValueOf(v).Kind()
		if kind != reflect.Int64 && kind != reflect.Int && kind != reflect.String && kind != reflect.Float64 {
			sendCmdError(conn, "val should be map of string -> int or string -> string, now is string -> "+kind.String())
			return false
		}

		cmd = fmt.Sprintf("ZADD %s %d %s", _redisValue.Key, v, k)
		sendCmd(conn, cmd)
		r, err := c.Do("ZADD", _redisValue.Key, v, k)
		if err != nil {
			sendCmdError(conn, err.Error())
			return false
		}
		sendCmdReceive(conn, r)
	}
	return true
}

// hset redis hset
func hset(conn *gosocketio.Channel, c redis.Conn, _redisValue redisValue) bool {
	vals, ok := (_redisValue.Val).(map[string]interface{})
	if !ok {
		sendCmdError(conn, "val should be map of string -> int64 or string -> string")
		return false
	}
	var cmd string
	for k, v := range vals {
		kind := reflect.ValueOf(v).Kind()
		if kind != reflect.Int64 && kind != reflect.Int && kind != reflect.String {
			sendCmdError(conn, "val should be map of string -> int or string -> string")
			return false
		}

		cmd = fmt.Sprintf("HSET %s %s %s", _redisValue.Key, k, v)
		sendCmd(conn, cmd)
		r, err := c.Do("HSET", _redisValue.Key, k, v)
		if err != nil {
			sendCmdError(conn, err.Error())
			return false
		}
		sendCmdReceive(conn, r)
	}
	return true
}

// AddRow add one row 2 redis
func AddRow(conn *gosocketio.Channel, data interface{}) {
	if c, _redisValue, ok := checkRedisValue(conn, data); ok {
		defer c.Close()
		t, err := keyType(conn, c, _redisValue.Key)
		if err != nil {
			sendCmdError(conn, err.Error())
			return
		}
		if t != _redisValue.T {
			sendCmdError(conn, "type "+_redisValue.T+" does not match"+t)
			return
		}
		var allowType = [4]string{
			"hash",
			"zset",
			"set",
			"list",
		}
		for i := 0; i < len(allowType); i++ {
			if t == allowType[i] {
				switch t {
				case "set", "list":
					var method string
					v, ok := (_redisValue.Val).(string)
					if ok == false {
						sendCmdError(conn, "val should be string")
						return
					}
					if t == "set" {
						method = "SADD"
					} else {
						method = "LPUSH"
					}
					cmd := method + " " + _redisValue.Key + " " + v
					sendCmd(conn, cmd)
					r, err := c.Do(method, _redisValue.Key, v)
					if err != nil {
						sendCmdError(conn, err.Error())
						return
					}
					sendCmdReceive(conn, r)
				case "zset":
					if zadd(conn, c, _redisValue) == false {
						return
					}
				case "hash":
					if hset(conn, c, _redisValue) == false {
						return
					}
				}
				conn.Emit("ReloadValue", 1)
				return
			}
		}
		sendCmdError(conn, "type "+t+" does not support")
	}
}
