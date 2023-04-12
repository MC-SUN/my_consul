package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// 向RegistryService发送post请求注册服务
func RegisterService(r Registration) error {
	//自检
	SetRegistryClient(r, 10*time.Second)
	//注册项的HeartbeatURL 和对应处理器函数绑定
	HeartbeatURL, err := url.Parse(r.HeartbeatURL)
	if err != nil {
		return err
	}
	http.HandleFunc(HeartbeatURL.Path, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	//注册项的Service·UpdateURL 和对应处理器函数绑定
	serviceUpdateURL, err := url.Parse(r.ServiceUpdateURL)
	if err != nil {
		return err
	}
	//为 DefaultServeMux路由多路复用注册路由及方法
	http.Handle(serviceUpdateURL.Path, &serviceUpdateHandler{})
	//缓冲区是具有读取和写入方法的可变大小的字节缓冲区,bytes.Buffer实现了io.Writer和io.Reader接口
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	err = enc.Encode(r) //调用后的buf就是序列化后的r
	if err != nil {
		return err
	}
	//发布向指定的 URL 发出 POST
	resp, err := http.Post(ServiceURL, "application/json", buf) //func Post(url string, contentType string, body io.Reader)(resp *Response, err error)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to register service with response: %v ", resp.Status)
	}
	return nil
}

// 向RegistryService发送delete请求
func ShutdownService(url string) error {
	req, err := http.NewRequest(http.MethodDelete, ServiceURL, bytes.NewBuffer([]byte(url)))
	if err != nil {
		return err
	}
	//text/plain的意思是将文件设置为纯文本的形式，浏览器在获取到这种文件时并不会对其进行处理。
	req.Header.Add("Content-Type", "text/plain")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to derigister service. RegistryService response: %v", res.Status)
	}
	return nil
}

type serviceUpdateHandler struct {
}

// 对应   sendRequiredServices，紧接着服务注册add之后，由注册服务端发送可用的provider信息patch给
func (s serviceUpdateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	var p patch
	//r.Body发送patch条目集合更新操作的json
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	fmt.Printf("We received an update:%v\n", p)
	prov.Update(p)
}

// 存储该服务（client文件对应单个服务，比如grade服务本地就有存储他的一些依赖项和可用的地址   所以需要注册服务器那里和该服务本地的providers进行同步）的所有可用的服务提供者urls，
// 消息来源就是在该服务post请求注册服务器的时候，在reg.add(rr)注册时，顺便扫描注册服务器的所有服务，找到所依赖项r.sendRequiredServices(reg)，
// 注册服务器会将所有能提供的服务信息打包patch 进行post请求  发送到该服务的ServiceUpdateURL中通知到被注册服务
// 该服务本地收到patch的post通知后会执行本地的providers的Update操作
type providers struct {
	services map[ServiceName][]string //键是服务名称，值是url列表，考虑所依赖的服务器是多机器部署
	mutex    *sync.RWMutex
}

var prov = providers{
	services: make(map[ServiceName][]string),
	mutex:    new(sync.RWMutex),
}

// 服务收到patch的post通知后会执行本地的providers的Update操作
// A-B-C
// C close, 则B收到通知的同时也要将自己的依赖变化也通知到依赖自己的项A，实际是通知到registry service，标记自己不完整。
func (p *providers) Update(pat patch) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	//新增操作，在该服务一开始注册自身的时候，或被notify
	for _, item := range pat.Added {
		if _, ok := p.services[item.Name]; !ok { //所依赖服务名称项还不存在,先添加名称项再添加对应url
			p.services[item.Name] = make([]string, 0)
		}
		//添加added
		p.services[item.Name] = append(p.services[item.Name], item.URL)
	}
	//删除操作,所依赖的服务不可用了，比如所依赖的服务突然宕机，服务注册服务器也会notify  发送patch的post通知告知，
	//这样就不会出现我使用那个服务，但是那个服务能不能用不知道，    服务注册服务器起到同步通知的作用，
	for _, item := range pat.Removed {
		if urls, ok := p.services[item.Name]; ok { //该服务存在,遍历该服务的url表，删去对应的url，注意 ！！！一个Name对应有多个url!!!
			for i := range urls {
				if urls[i] == item.URL {
					p.services[item.Name] = append(urls[:i], urls[i+1:]...)
				}
			}
		}
		//添加added
		p.services[item.Name] = append(p.services[item.Name], item.URL)
	}
}

// 通过服务提供者的名称找到服务对应url中的！！！一个！！！,简易负载均衡，随机数返回该名称服务的一个可用的url
func (p providers) get(name ServiceName) (string, error) {
	urls, ok := p.services[name]
	if !ok {
		return "", fmt.Errorf("no available url for service %v", name)
	}
	//简易负载均衡，随机数
	idx := int(rand.Float32() * float32(len(urls)))
	return urls[idx], nil //随机返回该名称服务的一个可用的url
}
func GetProvider(name ServiceName) (string, error) {
	return prov.get(name)
}

// 服务通过服务提供者进行定期自检，提示缺少的服务项
func (p providers) checkRequired(r Registration, freq time.Duration) {
	for {
		//ans := true
		//anss := "true"
		for _, need := range r.RequiredServices {
			if urls, ok := p.services[need]; !ok || len(urls) == 0 { //该依赖服务不存在于provider中,两种情况，provider根本没进行过新增操作||进行过删除操作且url列表空了
				log.Printf("Missing Required Service %v\n", need)
				//ans = false
			}
		}
		//if ans == false {
		//	anss = "false"
		//}
		//params := url.Values{}
		//params.Set("url", r.ServiceURL)
		//params.Set("ishealth", anss)
		//urlWithParams := ServiceURL + "?" + params.Encode()
		//_, err := http.Get(urlWithParams)
		//if err != nil {
		//	fmt.Println(err)
		//}
		time.Sleep(freq)
	}
}

var oncetime sync.Once
var registration Registration

// 设置RegistryService 客户端的本地配置  如设置服务缺失自检，heartbeat方法只调用一次， 单开协程无限循环
func SetRegistryClient(r Registration, freq time.Duration) {
	oncetime.Do(func() {
		registration = r
		go prov.checkRequired(r, freq)
	})
}
