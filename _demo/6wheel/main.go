/**
 * 每一次转动抽奖，后端计算出这次抽奖的中奖情况，并返回对应的奖品信息
 * 线程不安全，因为获奖概率低，并发更新库存的冲突很少能出现
 * 压力测试
 * wrk -t10 -c100 -d5 "http://localhost:8080/prize"
 */
package main

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kataras/iris"
	"github.com/kataras/iris/mvc"
)

// 奖品中奖概率
type Prate struct {
	Rate  int    // 万分之N的中奖概率
	Total int    // 总数量限制， 0表示无限数量
	CodeA int    // 中奖概率起始编码 (包含)
	CodeB int    // 中奖概率终止编码 (包含)
	Left  *int32 // 剩余数
}

// 方案一：加互斥锁
//var mu = sync.Mutex{}
// 方案二：atomic int32原子性操作
// atomic利用cpu的特性比互斥锁需要锁总线的效率高很多

var prizeList = []string{
	"一等奖, 火星单程船票",
	"二等奖, 南极之旅行",
	"三等奖, iphone一部",
	"", //没有中奖
}

var left = int32(1000)
var rateList = []Prate{
	//{1, 1, 0, 0, 1},
	//{2, 2, 1, 2, 2},
	//{5, 10, 3, 5, 10},
	{100, 1000, 0, 9999, &left},
}

type lotteryController struct {
	Ctx iris.Context
}

func newApp() *iris.Application {
	app := iris.New()
	mvc.New(app.Party("/")).Handle(&lotteryController{})
	return app
}

func main() {
	app := newApp()
	app.Run(iris.Addr(":8080"))
}

// http://localhost:8080/
func (c *lotteryController) Get() string {
	c.Ctx.Header("Content-Type", "text/html")
	return fmt.Sprintf("大转盘奖品列表:<br/> %s", strings.Join(prizeList, "<br/>\n"))
}

func (c *lotteryController) GetDebug() string {
	return fmt.Sprintf("获奖概率: %v\n", rateList)
}

func (c *lotteryController) GetPrize() string {
	// 第一步， 抽奖，根据随机数匹配奖品
	seed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(seed))
	code := r.Intn(10000)
	var myprize string
	var prizeRate *Prate

	//从奖品列表中匹配是否中奖
	for i, prize := range prizeList {
		rate := &rateList[i]
		if code >= rate.CodeA && code <= rate.CodeB {
			// 满足中奖条件
			myprize = prize
			prizeRate = rate
			break
		}
	}

	if myprize == "" {
		myprize = "很遗憾再来一次吧"
		return myprize
	}

	// 第二部 中奖了，开始要发奖
	if prizeRate != nil {
		if prizeRate.Total == 0 {
			// 无限量奖品
			log.Println("获奖情况:" + myprize)
			return myprize
		} else if *prizeRate.Left > 0 {
			//prizeRate.Left -= 1
			left := atomic.AddInt32(prizeRate.Left, -1)
			if left >= 0 {
				log.Println("获奖情况:" + myprize)
				return myprize

			}
		}
		myprize = "很遗憾再来一次吧"
		return myprize
	}
	return "很遗憾再来一次吧"
}
