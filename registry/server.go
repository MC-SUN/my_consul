package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

const ServerPort = ":3000"
const ServiceURL = "http://localhost" + ServerPort + "/services"

type registry struct {
	Registrations []Registration
	mutex         *sync.RWMutex //读写锁
}

// 服务注册时，顺便向服务所依赖项进行请求
func (r *registry) add(reg Registration) error {
	//注册该服务到注册服务器registry集合
	r.mutex.Lock() //写锁
	r.Registrations = append(r.Registrations, reg)
	r.mutex.Unlock()
	//并向注册服务器请求该服务所依赖的服务项
	err := r.sendRequiredServices(reg)
	//该服务可能会作为依赖项给其他服务使用
	//对于服务注册 服务器来说，所有的服务注册过来都是资源，所以如果有新服务注册过来newComePatch，就询问一下所有已注册的服务Registrations：新来了服务，你们需不需要该服务，
	//将新注册的服务作为参数传入，询问其他人需不需要newComePatch
	r.notify(patch{
		Added: []servicePatch{{
			Name: reg.ServiceName,
			URL:  reg.ServiceURL,
		}},
		Removed: nil,
	})
	return err
}

// 服务取消注册
func (r *registry) remove(url string) error {
	for i, j := range r.Registrations {
		if j.ServiceURL == url {
			//通知其他服务，该服务移除了！
			r.notify(patch{
				Added:   nil,
				Removed: []servicePatch{{Name: j.ServiceName, URL: j.ServiceURL}},
			})
			r.mutex.Lock()
			r.Registrations = append(r.Registrations[:i], r.Registrations[i+1:]...)
			r.mutex.Unlock()
			return nil
		}
	}
	return fmt.Errorf("service at url %v not found", url) //其他服务先于registryservice服务启动，又在registryservice服务启动后g关闭导致url不在Registrations中
}

// 将所有能提供的服务信息打包patch 通知给被注册服务，告诉它，我们有哪些服务可以提供给你
// 发送信息到被注册服务registration的ServiceUpdateURL
func (r registry) sendRequiredServices(registration Registration) error {
	r.mutex.RLock() //加读锁
	defer r.mutex.RUnlock()
	var p patch
	for _, hadPatch := range r.Registrations {
		for _, requiredPatch := range registration.RequiredServices {
			if hadPatch.ServiceName == requiredPatch {
				//如果能找到依赖服务项 ，加入到patch.added中
				p.Added = append(p.Added, servicePatch{
					Name: hadPatch.ServiceName,
					URL:  hadPatch.ServiceURL},
				)
			}
		}
	}
	err := r.sendPatch(p, registration.ServiceUpdateURL)
	if err != nil {
		return err
	}
	return nil
}

// 将依赖项信息的更新情况patch通过序列化json通知返回给被注册服务，被注册服务可以知道在服务注册中心哪些依赖有，哪些没有、
// 对应该服务的post请求处理Handler    (ServiceUpdateURL,Handler)
func (r registry) sendPatch(p patch, url string) error {
	//json.Marshal: obj		to	[]byte
	d, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = http.Post(url, "application/json", bytes.NewBuffer(d))
	if err != nil {
		return nil
	}
	return nil
}

func (r registry) notify(UpdatePatch patch) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	for _, reg := range r.Registrations { //向所有已经注册的服务询问
		go func(reg Registration) {
			for _, regNeed := range reg.RequiredServices { //可能传来的是多个服务的更新，所以针对每个依赖项都要notify..
				p := patch{
					Added:   nil,
					Removed: nil,
				}
				sendUpdate := false
				for _, newComeHave := range UpdatePatch.Added {
					if regNeed == newComeHave.Name { ////通知有一个该名称的服务启动了
						p.Added = append(p.Added, newComeHave)
						sendUpdate = true
					}
				}
				for _, justRemovedIs := range UpdatePatch.Removed {
					if regNeed == justRemovedIs.Name { //通知有一个该名称的服务停止了
						p.Removed = append(p.Removed, justRemovedIs)
						sendUpdate = true
					}
				}
				//reg依赖项有变化
				if sendUpdate {
					err := r.sendPatch(p, reg.ServiceUpdateURL)
					if err != nil {
						log.Println(err)
						return
					}
				}
			}
		}(reg)
	}
}

// 心跳检测，定时对已注册服务进行心跳检测
func (r *registry) heartbeat(freq time.Duration) {
	for {
		var wg sync.WaitGroup
		for _, reg := range r.Registrations {
			wg.Add(1)
			go func(reg Registration) {
				defer wg.Done()
				success := true
				//进行三次监测重试
				for try := 0; try < 3; try++ {
					//get请求
					resp, err := http.Get(reg.HeartbeatURL)
					if err != nil {
						fmt.Println(err)
					} else if resp.StatusCode == http.StatusOK {
						//为了区分同名多部署的服务，加上url
						log.Printf("HEARTBEAT OK. heartbeat check for %v with %v succeed.", reg.ServiceName, reg.ServiceURL)
						if !success {
							success = true
						}
						break
					}
					//三次
					log.Printf("HEARTBEAT FAILED. heartbeat check for %v with %v failed.", reg.ServiceName, reg.ServiceURL)
					if success {
						success = false
					}
					time.Sleep(1 * time.Second)
				}
				if !success {
					//三次监测都不成功
					r.remove(reg.ServiceURL)
				}
			}(reg)
		}
		wg.Wait()
		//每隔freq进行一次整体的循环健康检查
		time.Sleep(freq)
	}
}

//func (r *registry) ishealthUpdate(name string, url string, ishealth string) {
//	for i, j := range r.Registrations {
//		if j.ServiceURL == url {
//			//找到该服务
//			r.mutex.Lock()
//			if ishealth=="true"{
//				j.RequiredOK=true
//			}else {
//				j.RequiredOK=false
//			}
//			r.mutex.Unlock()
//			return nil
//		}
//	}
//}

var once sync.Once

// 设置RegistryService  如设置心跳检测频率，heartbeat方法只调用一次， 单开协程无限循环
func SetupRegistryService() {
	once.Do(func() {
		go reg.heartbeat(3 * time.Second)
	})
}

// 实例化registry全局变量
var reg = registry{
	Registrations: make([]Registration, 0),
	mutex:         new(sync.RWMutex),
}

// RegistryService 注册服务的webservice 实现type Handler interface
// 注册服务处理器Handler
//
//	即type Handler interface {
//		ServeHTTP(ResponseWriter, *Request)
//	}
type RegistryService struct {
}

// 客户端访问注册服务器，使用post进行注册reg.add(rr) //注册  和delete移除注册操作
func (receiver RegistryService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("Request received")
	switch r.Method {
	//注册使用post
	case http.MethodPost:
		dec := json.NewDecoder(r.Body) //NewDecoder 返回从 r 读取的新解码器。解码器引入了自己的缓冲
		var rr Registration            //区分r *http.Request
		err := dec.Decode(&rr)         //JSON 转换为 Go 结构体类型值
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("Adding Service %v with URL:%v", rr.ServiceName, rr.ServiceURL)
		err = reg.add(rr) //注册
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	case http.MethodDelete:
		reqDelete, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		url := string(reqDelete)
		log.Printf("removing service at url: %s ", url)
		err = reg.remove(url)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	//	//对应服务自检get返回，来通知服务是否完整
	//case http.MethodGet:
	//	resp := r.URL.Query()
	//	name := resp.Get("name")
	//	url := resp.Get("url")
	//	ishealth := resp.Get("ishealth")
	//	reg.ishealthUpdate(name, url, ishealth)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

}
