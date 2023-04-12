package main

import (
	"context"
	"fmt"
	stlog "log"
	"mydistributed/grades"
	"mydistributed/log"
	"mydistributed/registry"
	"mydistributed/service"
)

func main() {
	host, port := "localhost", "6000"
	serviceAddress := fmt.Sprintf("http://%v:%v", host, port)

	r := registry.Registration{
		ServiceName:      registry.GradingService,
		ServiceURL:       serviceAddress,
		RequiredServices: []registry.ServiceName{registry.LogService},
		ServiceUpdateURL: serviceAddress + "/services",
		HeartbeatURL:     serviceAddress + "/heartbeat",
	}
	ctx, err := service.Start(context.Background(),
		r,
		host,
		port,
		grades.RegisterHandlers)
	if err != nil {
		stlog.Fatal(err)
	}
	//找到所有需要的依赖服务
	//1.log服务
	if logProvider, err := registry.GetProvider(registry.LogService); err == nil {
		fmt.Printf("Logging service found at: %s\n", logProvider) //
		//客户端的log包或者log库的函数，
		//找到服务了，即服务注册服务器里有log服务，本地设置log服务SetClientLogger
		log.SetClientLogger(logProvider, r.ServiceName)
	}

	<-ctx.Done()
	fmt.Println("Shutting down grading service")
}
