package main

import (
	"encoding/json"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"golang.org/x/net/websocket"
	"html/template"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

type User struct {
	Username string
}

type RespData struct {
	Message
	Users []string `json:"users"`
}


// 全局信息
var users map[*websocket.Conn]string
var redisAddress = []string{"tcp", "127.0.0.1:6379"}
var c redis.Conn
func main() {
	var err error
	if c == nil {
		// 根据不同环境选择redis连接方式
		if os.Getenv("REDIS_PORT") != "" {
			redisAddress = strings.Split(os.Getenv("REDIS_PORT"), "://")
		}
		// 连接redis
		c, err = redis.Dial(redisAddress[0], redisAddress[1])
		if err != nil{
			panic("redis连接失败" + err.Error())
			return
		}
		// 选取1号数据库 blog为0号数据库
		if _, err = c.Do("SELECT", 1); err != nil {
			panic("redis数据库选择失败" + err.Error())
			return
		}
	}
	// 初始化数据
	users = make(map[*websocket.Conn]string)

	// 渲染页面
	http.HandleFunc("/", index)

	// 监听socket方法
	http.Handle("/webSocket", websocket.Handler(webSocket))

	// 监听8080端口
	http.ListenAndServe(":8889", nil)
}

func index(w http.ResponseWriter, r *http.Request)  {
	// 查出所有用户
	res, err := redis.ByteSlices(c.Do("SMEMBERS", "users"))
	if err != nil{
		panic(err)
		return
	}
	user := make([]string,0)
	for _,v := range res{
		user = append(user, string(v))
	}
	t,_ := template.ParseFiles("index.html")
	t.Execute(w, user)
}

func webSocket(ws *websocket.Conn)  {
	var message Message
	var data string
	for {
		// 接收数据
		err := websocket.Message.Receive(ws, &data)
		if err != nil {
			// 移除出错的连接
			delete(users, ws)
			// 删除redis用户集合
			c.Do("SREM","users", message.Username)
			fmt.Println("连接异常")
			break
		}
		// 解析信息
		err = json.Unmarshal([]byte(data), &message)
		if err != nil {
			fmt.Println("解析数据异常")
		}

		// 添加新用户到map中,已经存在的用户不必添加
		if _, ok := users[ws]; !ok {
			users[ws] = message.Username
		}
		// redis数据库添加用户
		userExist, _ := redis.Bool(c.Do("SISMEMBER", "users", message.Username))
		if !userExist {
			_, err := c.Do("SADD", "users", message.Username)
			if err != nil {
				panic("redis添加数据出错: " + err.Error())
				return
			}
		}
		// 将用户每个人的消息存储到对应的集合message
		value := "username:" + message.Username+
					"-time:" + time.Now().Format("2006-1-2-15:04:05") +
					"-rand:" + strconv.Itoa(rand.Intn(1000))+
					"-message:" + message.Message
		_,	err =c.Do("SADD", "message", value)
		if err != nil{
			panic("保存记录异常" + err.Error())
			return
		}
		
		// 通过webSocket将当前信息分发
		var respData RespData
		json.Unmarshal([]byte(data), &respData)
		res, _ := redis.ByteSlices(c.Do("SMEMBERS", "users"))
		for _, v := range res{
			respData.Users = append(respData.Users, string(v))
		}
		data,_ := json.Marshal(respData)
		for key := range users{
			err := websocket.Message.Send(key, string(data))
			if err != nil{
				// 移除出错的连接
				delete(users, key)
				// 删除redis用户集合
				c.Do("SREM","users", message.Username)
				fmt.Println("发送出错: " + err.Error())
				break
			}
		}
	}
}
