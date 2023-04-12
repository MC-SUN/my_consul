package log

import (
	"io"
	stdlog "log"
	"net/http"
	"os"
)

var log *stdlog.Logger

type fileLog string //一个文件log类型,实现io.Writer接口 ,写入消息到文件

func (fl fileLog) Write(p []byte) (int, error) { //实现io.Writer接口
	f, err := os.OpenFile(string(fl), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600) //O_CREATE如果不存在，则创建新文件
	if err != nil {
		return 0, err
	}
	defer f.Close()

	return f.Write(p) //写入将 len（b） 字节从 b 写入文件。它返回写入的字节数和错误（如果有
}

// 使用go原生log库实例化Logger,dest是log文件路径,设置记录器是io.Writer类型,而fileLog实现了io.Writer接口
// log是一个很简洁的日志库，它有三种日志输出方式print、panic、fatal，且可以自己定制日志的输出格式
// log库默认使用的std实例是事先初始化好的，那么借助New方法，我们也可以定制自己的logger：
func Run(dest string) { //实例化Logger 路径dest
	//func New(out io.Writer, prefix string, flag int) *Logger
	log = stdlog.New(fileLog(dest), "go——log: ", stdlog.LstdFlags)
}
func RegisterHandles() {
	//HandleFunc 在 DefaultServeMux 中注册给定模式的处理程序函数
	//在这里集中添加handler!!!!!!!!!!!!!!!!!!!!!!!!!!
	//1. /log
	http.HandleFunc("/log", func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodPost:
			msg, err := io.ReadAll(request.Body)
			if err != nil || len(msg) == 0 {
				writer.WriteHeader(http.StatusBadRequest) //报错或请求体为空
				return
			}
			write(string(msg))
		default:
			writer.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	//2. ........
}

func write(s string) {
	log.Printf(s) //Printf (l *Logger)调用 l.Output 以打印到记录器。参数是以自定义的io.Writer实现处理，而不是默认的 fmt 的方式处理
}
