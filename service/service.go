package service

import (
	"context"
	"fmt"
	"log"
	"mydistributed/registry"
	"net/http"
)

// 使用函数将服务统一管理，统一启动，并使用registry.RegisterService运行注册，并管理服务，建立WithCancel上下文，并返回ctx
func Start(ctx context.Context, reg registry.Registration, host, port string, registerHandleFunc func()) (context.Context, error) {
	registerHandleFunc()                                 //运行HandleFunc注册事件，HandleFunc 在 DefaultServeMux 中注册给定模式的处理程序函数
	ctx = startService(ctx, reg.ServiceName, host, port) //启动服务
	err := registry.RegisterService(reg)                 //RegisterService函数向RegistryService发送post请求来注册服务
	if err != nil {
		return ctx, err
	}
	return ctx, nil
}

func startService(ctx context.Context, serviceName registry.ServiceName, host string, port string) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	//WithCancel 返回具有 Done的channel的父级副本。
	// 取消此上下文会释放与其关联的资源，因此代码应在此上下文中运行的操作完成后立即调用 cancel。
	srv := http.Server{} //使用默认defaultmultimux
	srv.Addr = ":" + port
	go func() {
		log.Println(srv.ListenAndServe())                                        //ListenAndServe如果有错会返回，否则会一直存在，不向下执行
		err := registry.ShutdownService(fmt.Sprintf("http://%s:%s", host, port)) //ShutdownService函数向RegistryService发送delete请求来取消注册
		if err != nil {
			log.Println(err)
		}
		cancel()
	}()
	go func() {
		fmt.Printf("%v started. Press any key to stop. \n", serviceName)
		var s string
		fmt.Scanln(&s)                                                           //等待输入
		err := registry.ShutdownService(fmt.Sprintf("http://%s:%s", host, port)) //ShutdownService函数向RegistryService发送delete请求来取消注册
		if err != nil {
			log.Println(err)
		}
		srv.Shutdown(ctx)
		cancel()
	}()
	return ctx
}
