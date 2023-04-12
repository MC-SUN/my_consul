package main

import (
	"context"
	"fmt"
	"log"
	"mydistributed/registry"
	"net/http"
)

func main() {
	//设置心跳检测等设置
	registry.SetupRegistryService()
	http.Handle("/services", &registry.RegistryService{})
	ctx, cancel := context.WithCancel(context.Background()) //使用Background初始化WithCancel ctx
	srv := http.Server{}
	srv.Addr = registry.ServerPort
	go func() {
		log.Println(srv.ListenAndServe()) //ListenAndServe如果有错会返回，否则会一直存在，不向下执行
		cancel()
	}()
	go func() {
		fmt.Printf("RegistryService started. Press any key to stop. \n")
		var s string
		fmt.Scanln(&s) //等待输入
		srv.Shutdown(ctx)
		cancel()
	}()
	<-ctx.Done() //阻塞直到接收信号，对应service里手动停止或http的服务器启动时出错的父级ctx，WithCancel 调用 cancel 时返回只读通道 Done
	fmt.Println("Shutting down Registry Service.")
}
