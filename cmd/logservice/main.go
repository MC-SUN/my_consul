package main

import (
	"context"
	"fmt"
	stdlog "log"
	"mydistributed/log"
	"mydistributed/registry"
	"mydistributed/service"
)

// 分布式服务，启动运行log服务的二进制程序
func main() {
	log.Run("./destributed.log") //初始化服务，参数等
	host, port := "localhost", "4000"
	serviceAddr := fmt.Sprintf("http://%s:%s", host, port)
	r := registry.Registration{
		ServiceName:      registry.LogService,
		ServiceURL:       serviceAddr,
		RequiredServices: make([]registry.ServiceName, 0),
		ServiceUpdateURL: serviceAddr + "/services",
		HeartbeatURL:     serviceAddr + "/heartbeat",
	}
	ctx, err := service.Start(
		context.Background(),
		r,
		host,
		port,
		log.RegisterHandles,
	)
	if err != nil {
		stdlog.Fatalln(err)
	}
	<-ctx.Done() //阻塞直到接收信号，对应service里手动停止或http的服务器启动时出错的父级ctx，WithCancel 调用 cancel 时返回只读通道 Done
	fmt.Println("Shutting down Log Service.")
}
