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
	"sync"
	"time"

	"github.com/kataras/iris"
	"github.com/kataras/iris/mvc"
)

//奖品类型 枚举值 iota 从0开始
const (
	giftTypeCoin      = iota //虚拟币
	giftTypeCoupon           //不同劵
	giftTypeCouponFix        //相同的劵
	giftTypeRealSmall        //实物小奖
	giftTypeRealLarge        //实物大奖
)

type gift struct {
	id       int      //奖品ID
	name     string   //奖品名称
	pic      string   //奖品图片
	link     string   //奖品连接
	gtype    int      //奖品类型
	data     string   //奖品的数据（特定的配置信息）
	dataList []string //奖品数据集合
	total    int      //总数， 0 不限量
	left     int      //剩余数量
	inuse    bool     //是否使用中
	rate     int      //中奖概率 万分之N， 0 - 9999
	rateMin  int      //大于等于最小中奖编码
	rateMax  int      //小于中奖编码
}

//最大的中奖号码
const rateMax = 10000

var logger *log.Logger

//奖品列表
var giftList []*gift

// 互斥锁
var mu sync.Mutex

type lotteryController struct {
	Ctx iris.Context
}

//初始化日志
func initLog() {
	f, _ := os.Create("/var/log/lottery_demo.log")
	logger = log.New(f, "", log.Ldate|log.Lmicroseconds)
}

//初始化奖品列表
func initGift() {
	giftList = make([]*gift, 5)
	g1 := gift{
		id:       1,
		name:     "手机大奖",
		pic:      "",
		link:     "",
		gtype:    giftTypeRealLarge,
		data:     "",
		dataList: nil,
		total:    20000,
		left:     20000,
		inuse:    true,
		rate:     10000,
		rateMin:  0,
		rateMax:  0,
	}
	giftList[0] = &g1

	g2 := gift{
		id:       2,
		name:     "充电器",
		pic:      "",
		link:     "",
		gtype:    giftTypeRealSmall,
		data:     "",
		dataList: nil,
		total:    5,
		left:     5,
		inuse:    false,
		rate:     100,
		rateMin:  0,
		rateMax:  0,
	}
	giftList[1] = &g2

	g3 := gift{
		id:       3,
		name:     "优惠券满200减50",
		pic:      "",
		link:     "",
		gtype:    giftTypeCouponFix,
		data:     "mall-coupon-2018",
		dataList: nil,
		total:    5,
		left:     5,
		inuse:    false,
		rate:     5000,
		rateMin:  0,
		rateMax:  0,
	}
	giftList[2] = &g3

	g4 := gift{
		id:       4,
		name:     "直降优惠券50元",
		pic:      "",
		link:     "",
		gtype:    giftTypeCoupon,
		data:     "",
		dataList: []string{"c01", "c02", "c03", "c04", "c05"},
		total:    5,
		left:     5,
		inuse:    false,
		rate:     2000,
		rateMin:  0,
		rateMax:  0,
	}
	giftList[3] = &g4

	g5 := gift{
		id:       5,
		name:     "金币",
		pic:      "",
		link:     "",
		gtype:    giftTypeCoin,
		data:     "10金币",
		dataList: nil,
		total:    5,
		left:     5,
		inuse:    false,
		rate:     5000,
		rateMin:  0,
		rateMax:  0,
	}
	giftList[4] = &g5

	//数据整理， 中奖区间数据, 提高效率
	rateStart := 0
	for _, data := range giftList {
		if !data.inuse {
			continue
		}
		data.rateMin = rateStart
		data.rateMax = rateStart + data.rate
		if data.rateMax >= rateMax {
			data.rateMax = rateMax
			rateStart = 0
		} else {
			rateStart += data.rate
		}
	}

}

func newApp() *iris.Application {
	app := iris.New()
	mvc.New(app.Party("/")).Handle(&lotteryController{})

	initLog()
	initGift()

	return app
}

func main() {
	app := newApp()
	app.Run(iris.Addr(":8080"))
}

// 奖品数量的信息 GET http://localhost:8080/
func (c *lotteryController) Get() string {
	count := 0
	//库存
	total := 0
	for _, data := range giftList {
		if data.inuse && (data.total == 0 ||
			(data.total > 0 && data.left > 0)) {
			count++
			total += data.left
		}
	}

	return fmt.Sprintf("当前有效奖品种类数量: %d,"+
		"限量奖品总数量=%d\n", count, total)
}

//抽奖 Get http://localhost:8080/lucky
func (c *lotteryController) GetLucky() map[string]interface{} {
	mu.Lock()
	defer mu.Unlock()
	code := luckCode()
	ok := false
	result := make(map[string]interface{})
	result["success"] = ok
	for _, data := range giftList {
		if !data.inuse || (data.total >= 0 && data.left <= 0) {
			continue
		}

		if data.rateMin <= int(code) && data.rateMax > int(code) {
			//中奖了，抽奖编码在奖品编码范围内
			//开始发奖
			sendData := ""
			switch data.gtype {
			case giftTypeCoin:
				ok, sendData = sendCoin(data)
			case giftTypeCoupon:
				ok, sendData = sendCoupon(data)
			case giftTypeCouponFix:
				ok, sendData = sendCouponFix(data)
			case giftTypeRealSmall:
				ok, sendData = sendRealSmall(data)
			case giftTypeRealLarge:
				ok, sendData = sendRealLarge(data)
			}

			if ok {
				// 中奖后，成功得到奖品，
				// 生成中奖记录
				saveLuckyData(code, data.id, data.name, data.link, sendData, data.left)
				result["success"] = ok
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

func sendCoin(data *gift) (bool, string) {
	if data.total == 0 {
		//数量无限
		return true, data.data
	} else if data.left > 0 {
		// 还有剩余
		data.left = data.left - 1
		return true, data.data
	} else {
		return false, "奖品已发完"
	}
}

//不同值的优惠券
func sendCoupon(data *gift) (bool, string) {
	if data.left > 0 {
		// 还有剩余
		left := data.left - 1
		data.left = left
		return true, data.dataList[left]
	} else {
		return false, "奖品已发完"
	}
}

func sendCouponFix(data *gift) (bool, string) {
	if data.total == 0 {
		//数量无限
		return true, data.data
	} else if data.left > 0 {
		// 还有剩余
		data.left = data.left - 1
		return true, data.data
	} else {
		return false, "奖品已发完"
	}
}

func sendRealSmall(data *gift) (bool, string) {
	if data.total == 0 {
		//数量无限
		return true, data.data
	} else if data.left > 0 {
		// 还有剩余
		data.left = data.left - 1
		return true, data.data
	} else {
		return false, "奖品已发完"
	}
}
func sendRealLarge(data *gift) (bool, string) {
	if data.total == 0 {
		//数量无限
		return true, data.data
	} else if data.left > 0 {
		// 还有剩余
		data.left = data.left - 1
		return true, data.data
	} else {
		return false, "奖品已发完"
	}
}

func saveLuckyData(code int32, id int, name string, link string, sendData string, left int) {
	logger.Printf("lucky, code=%d, gift=%d, name=%s, link=%s, data=%s, left=%d",
		code, id, name, link, sendData, left)
}
