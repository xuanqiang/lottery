/**
 * 设置红包
 * curl "http://localhost:8080/set?uid=1&money=100&num=100"
 * 抢红包
 * curl "http://localhost:8080/get?id=1162763031&uid=1"
 * 并发压力测试：
 * wrk -t10 -c10 -d6 "http://localhost:8080/set?uid=1&money=100&num=100"
 */
package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/kataras/iris"
	"github.com/kataras/iris/mvc"
)

type task struct {
	id       uint32
	callback chan uint
}

// 红包列表,在高并发时会出现异常fatal error: concurrent map writes
//var packageList map[uint32][]uint = make(map[uint32][]uint)
// 线程安全
var packageList *sync.Map = new(sync.Map)

//单个消息队列 (用队列的方式解决map中list抽取红包后更新list的并发安全性)
//var chTasks chan task = make(chan task) //定义一个队列
// 多个消息队列
const taskNum = 16

var chanTaskList []chan task = make([]chan task, taskNum)

type lotteryController struct {
	Ctx iris.Context
}

func newApp() *iris.Application {
	app := iris.New()
	mvc.New(app.Party("/")).Handle(&lotteryController{})
	for i := 0; i < taskNum; i++ {
		chanTaskList[i] = make(chan task)
		go fetchPackageListMoney(chanTaskList[i])
	}

	return app
}

func main() {
	app := newApp()
	app.Run(iris.Addr(":8080"))
}

// 返回全部红包地址 红包数量和金额
// Get http://localhost:8080/
func (c *lotteryController) Get() map[uint32][2]int {
	rs := make(map[uint32][2]int)
	//for id, list := range packageList {
	//	var money int
	//	for _, v := range list {
	//		money += int(v)
	//	}
	//	rs[id] = [2]int{len(list), money}
	//}
	packageList.Range(func(key, value interface{}) bool {
		id := key.(uint32)
		list := value.([]uint)
		var money int
		for _, v := range list {
			money += int(v)
		}
		rs[id] = [2]int{len(list), money}
		return true
	})
	return rs
}

// 发送红包
// http://localhost:8080/set?uid=1&money=100&num
func (c *lotteryController) GetSet() string {
	uid, errUid := c.Ctx.URLParamInt("uid")
	money, errMoney := c.Ctx.URLParamFloat64("money")
	num, errNum := c.Ctx.URLParamInt("num")
	if errUid != nil || errMoney != nil || errNum != nil {
		return fmt.Sprintf("参数格式异常, errUid=%d, errMoney=%d, errNum=%d\n",
			errUid, errMoney, errNum)
	}

	moneyTotal := int(money * 100)
	if uid < 1 || moneyTotal < num || num < 1 {
		return fmt.Sprintf("参数数值异常, uid=%d, money=%d, num=%d\n",
			uid, money, num)
	}

	//金额分配算法
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	rMax := 0.55 //随机分配的最大值
	if num > 1000 {
		rMax = 0.01
	} else if num >= 100 {
		rMax = 0.1
	} else if num >= 10 {
		rMax = 0.3
	}
	list := make([]uint, num)
	leftMoney := moneyTotal
	leftNum := num
	//大循环开始, 分配金额给到每一个红包
	for leftNum > 0 {
		if leftNum == 1 {
			// 最后一个红包， 剩余的全部金额都给它
			list[num-1] = uint(leftMoney)
			break
		}
		//例如剩下4分钱还有4个红包
		if leftMoney == leftNum {
			for i := num - leftNum; i < num; i++ {
				list[i] = 1
			}
			break
		}

		rMoney := int(float64(leftMoney-leftNum) * rMax)
		m := r.Intn(rMoney)
		if m < 1 {
			m = 1
		}
		list[num-leftNum] = uint(m)
		leftMoney -= m
		leftNum--
	}

	// 红包的唯一ID
	id := r.Uint32()
	//packageList[id] = list
	packageList.Store(id, list)
	// 返回抢红包的url
	return fmt.Sprintf("/get?id=%d&uid=%d&num=%d\n", id, uid, num)
}

// 抢红包
// http://localhost:8080/get?id=1&uid=1
func (c *lotteryController) GetGet() string {
	uid, errUid := c.Ctx.URLParamInt("uid")
	id, errId := c.Ctx.URLParamInt("id")
	if errUid != nil || errId != nil {
		return fmt.Sprintf("")
	}
	if uid < 1 || id < 1 {
		return fmt.Sprintf("参数错误")
	}
	//list, ok := packageList[uint32(id)]
	list1, ok := packageList.Load(uint32(id))
	list := list1.([]int)
	if !ok || len(list) < 1 {
		return fmt.Sprintf("红包不存在, id=%d\n", id)
	}
	// 构造一个抢红包的任务
	callback := make(chan uint)
	t := task{id: uint32(id), callback: callback}
	// 发送任务
	chTasks := chanTaskList[id%taskNum]
	chTasks <- t
	// 接受返回结果
	money := <-callback
	if money <= 0 {
		return "很遗憾，没有抢到红包\n"
	} else {
		return fmt.Sprintf("恭喜你抢到一个红包， 金额为:%d\n", money)
	}
}

func fetchPackageListMoney(chTasks chan task) {
	for {
		t := <-chTasks
		id := t.id
		l, ok := packageList.Load(id)
		if ok && l != nil {
			list := l.([]int) //类型转换
			// 分配一个随机数
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			i := r.Intn(len(list))
			money := list[i]
			// 更新红包列表中的信息
			if len(list) > 1 {
				if i == len(list)-1 {
					//packageList[uint32(id)] = list[:i]
					packageList.Store(uint32(id), list[:i])
				} else if i == 0 {
					//packageList[uint32(id)] = list[1:]
					packageList.Store(uint32(id), list[1:])
				} else {
					//packageList[uint32(id)] = append(list[:i], list[i+1:]...)
					packageList.Store(uint32(id), append(list[:i], list[i+1:]...))
				}
			} else {
				//delete(packageList, uint32(id))
				packageList.Delete(uint32(id))
			}

			t.callback <- uint(money)
		} else {
			t.callback <- 0
		}

	}
}
