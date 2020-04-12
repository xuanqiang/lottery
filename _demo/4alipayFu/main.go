/**
 * 微信摇一摇
 * 基础功能：
 * /lucky 只有一个抽奖的接口
 * 压力测试：
 * wrk -t10 -c10 -d5 http://localhost:8080/lucky  -t 线程数  -c 连接数 -d时间5s
 */
package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kataras/iris/mvc"

	"github.com/kataras/iris"
)

type gift struct {
	id      int    //奖品ID
	name    string //奖品名称
	pic     string //奖品图片
	link    string //奖品连接
	inuse   bool   //是否使用中
	rate    int    //中奖概率 万分之N， 0 - 9999
	rateMin int    //大于等于最小中奖编码
	rateMax int    //小于中奖编码
}

//最大的中奖号码
const rateMax = 10

var logger *log.Logger

type lotteryController struct {
	Ctx iris.Context
}

//初始化日志
func initLog() {
	f, _ := os.Create("/var/log/lottery_demo.log")
	logger = log.New(f, "", log.Ldate|log.Lmicroseconds)
}

func newApp() *iris.Application {
	app := iris.New()
	mvc.New(app.Party("/")).Handle(&lotteryController{})

	initLog()

	return app
}

func main() {
	app := newApp()
	app.Run(iris.Addr(":8080"))
}

//设定为数组
func newGift() *[5]gift {
	giftList := new([5]gift)
	g1 := gift{
		id:      1,
		name:    "富强福",
		pic:     "富强福.jpg",
		link:    "",
		inuse:   true,
		rate:    0,
		rateMin: 0,
		rateMax: 0,
	}
	giftList[0] = g1

	g2 := gift{
		id:      2,
		name:    "和谐福",
		pic:     "和谐福.jpg",
		link:    "",
		inuse:   true,
		rate:    0,
		rateMin: 0,
		rateMax: 0,
	}
	giftList[1] = g2

	g3 := gift{
		id:      3,
		name:    "友善福",
		pic:     "友善福.jpg",
		link:    "",
		inuse:   true,
		rate:    0,
		rateMin: 0,
		rateMax: 0,
	}
	giftList[2] = g3

	g4 := gift{
		id:      4,
		name:    "爱国福",
		pic:     "爱国福.jpg",
		link:    "",
		inuse:   true,
		rate:    0,
		rateMin: 0,
		rateMax: 0,
	}
	giftList[3] = g4

	g5 := gift{
		id:      5,
		name:    "敬业福",
		pic:     "敬业福.jpg",
		link:    "",
		inuse:   true,
		rate:    0,
		rateMin: 0,
		rateMax: 0,
	}
	giftList[4] = g5

	return giftList
}

func giftRange(rate string) *[5]gift {
	giftList := newGift()
	rates := strings.Split(rate, ",")
	ratesLen := len(rates)
	//数据整理， 中奖区间数据, 提高效率
	rateStart := 0
	//data是数值类型
	for i, data := range giftList {
		if !data.inuse {
			continue
		}
		grate := 0
		if i < ratesLen {
			grate, _ = strconv.Atoi(rates[i])
		}
		giftList[i].rate = grate
		giftList[i].rateMin = rateStart
		giftList[i].rateMax = rateStart + grate
		if giftList[i].rateMax >= rateMax {
			giftList[i].rateMax = rateMax
			rateStart = 0
		} else {
			rateStart += grate
		}
	}
	fmt.Printf("giftlist=%v\n", giftList)
	return giftList
}

// GET http://localhost:8080
func (c *lotteryController) Get() string {
	rate := c.Ctx.URLParamDefault("rate", "4, 3, 2, 1, 0")
	giftList := giftRange(rate)
	return fmt.Sprintf("%v\n", giftList)
}

//抽奖 Get  http://localhost:8080/lucky?uid=1&rate=4,3,2,1,0
func (c *lotteryController) GetLucky() map[string]interface{} {
	uid, _ := c.Ctx.URLParamInt("uid")
	rate := c.Ctx.URLParamDefault("rate", "4, 3, 2, 1, 0")
	code := luckCode()
	ok := true
	result := make(map[string]interface{})
	result["success"] = ok
	giftList := giftRange(rate)
	for _, data := range giftList {
		if !data.inuse {
			continue
		}

		if data.rateMin <= int(code) && data.rateMax > int(code) {
			//中奖了，抽奖编码在奖品编码范围内
			//开始发奖
			sendData := data.pic
			if ok {
				// 中奖后，成功得到奖品，
				// 生成中奖记录
				saveLuckyData(code, data.id, data.name, data.link, sendData)
				result["success"] = ok
				result["uid"] = uid
				result["id"] = data.id
				result["name"] = data.name
				result["link"] = data.link
				result["data"] = sendData
				break
			}
		}
	}
	return result
}

//抽奖阶段
func luckCode() int32 {
	//生成时间戳
	seed := time.Now().UnixNano()
	code := rand.New(rand.NewSource(seed)).Int31n(rateMax)
	return code
}

func saveLuckyData(code int32, id int, name string, link string, sendData string) {
	logger.Printf("lucky, code=%d, gift=%d, name=%s, link=%s, data=%s",
		code, id, name, link, sendData)
}
