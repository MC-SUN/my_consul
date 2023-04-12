package registry

// 注册项，用来注册服务和服务中心管理服务
type Registration struct {
	ServiceName      ServiceName
	ServiceURL       string
	RequiredServices []ServiceName //该服务的依赖项集合，所依赖的其他服务：如grade service 依赖log service
	ServiceUpdateURL string        //是自己服务的serviceAddress + "/services",
	// 将依赖项信息的更新情况patch通过序列化json通知返回给被注册服务，被注册服务可以知道在服务注册中心哪些依赖有，哪些没有、
	// 对应被注册服务的post请求处理Handler    (ServiceUpdateURL,Handler)
	HeartbeatURL string //心跳检测的地址 接受心跳检测的get请求   是自己服务的serviceAddress + "/heartbeat"
	RequiredOK   bool
}
type ServiceName string //类型定义

const (
	LogService     = ServiceName("LogService") //全局只读变量
	GradingService = ServiceName("GradingService")
)

// 单个条目
type servicePatch struct {
	Name ServiceName
	URL  string
}

// 条目集合 即动态对比单个Registration的RequireServices和当前服务注册中心所已经注册的服务条目，看是否存在
type patch struct {
	Added   []servicePatch
	Removed []servicePatch
}
