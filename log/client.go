package log

//为其他服务创建api服务
import (
	"bytes"
	"fmt"
	stlog "log"
	"mydistributed/registry"
	"net/http"
)

// 要想使用log服务器，客户端需要进行如下设置！！！
// 第一个参数是对应log服务的具体url，第二个是调用log服务的服务名称，
// 客户端调用此方法，将客户端那边的环境stlog的设置改变，如将客户端那边的stlog.SetOutput(&clientLogger{url: serviceURL})设置为日志服务来代理
func SetClientLogger(serviceURL string, clientService registry.ServiceName) {
	//设置前缀，即效果是log日志中：服务名称-对应日志,将这些整体内容text/plain格式发给log服务器
	stlog.SetPrefix(fmt.Sprintf("[%v] - ", clientService))
	stlog.SetFlags(0)                               //时间戳
	stlog.SetOutput(&clientLogger{url: serviceURL}) //设置输出设置标准记录器的输出目标，即将内容写到url对应的log服务中，即Post请求
}

// clientLogger实现io.Writer接口并在重写的Write方法中封装post请求，发送日志到日志服务器
type clientLogger struct {
	url string
}

// 实现io.Writer
func (cl clientLogger) Write(data []byte) (int, error) {
	b := bytes.NewBuffer([]byte(data))
	res, err := http.Post(cl.url+"/log", "text/plain", b)
	if err != nil {
		return 0, err
	}
	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Failed to send log message. Service responded with %d - %s", res.StatusCode, res.Status)
	}
	return len(data), nil
}
