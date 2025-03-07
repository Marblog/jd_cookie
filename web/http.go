package web

import (
	"embed"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/memstore"
	"github.com/gin-gonic/gin"
	"github.com/scjtqs/jd_cookie/config"
	"github.com/scjtqs/jd_cookie/web/repo"
	log "github.com/sirupsen/logrus"
	"go.uber.org/dig"
	"html/template"
	"net/http"
	"os"
	"time"
)

type httpServer struct {
	engine      *gin.Engine
	HTTP        *http.Server
	ct          *dig.Container
	Conf        *config.Conf
	cookiesRepo repo.CookiesRepository
}

var HTTPServer = &httpServer{}

func (s *httpServer) Run(addr string, ct *dig.Container) {
	s.ct = ct
	ct.Invoke(func(conf *config.Conf) {
		s.Conf = conf
	})
	var f embed.FS
	ct.Invoke(func(file embed.FS) {
		f = file
	})
	gin.SetMode(gin.ReleaseMode)
	s.engine = gin.New()
	// 创建基于 内存 的存储引擎，secret 参数是用于加密的密钥
	store := memstore.NewStore([]byte("scjtqsnb"))
	// 设置session中间件，参数mysession，指的是session的名字，也是cookie的名字
	// store是前面创建的存储引擎，我们可以替换成其他存储引擎
	s.engine.Use(sessions.Sessions("mysession", store))

	s.engine.Use(func(c *gin.Context) {
		if c.Request.Method != "GET" && c.Request.Method != "POST" {
			log.Warnf("已拒绝客户端 %v 的请求: 方法错误", c.Request.RemoteAddr)
			c.Status(404)
			return
		}
		c.Next()
	})
	// 自动加载模板
	t := template.New("tmp")
	//func 函数映射 全局模板可用
	t.Funcs(template.FuncMap{
		"getYear":        GetYear,
		"formatAsDate":   FormatAsDate,
		"getDate":        GetDate,
		"getavator":      Getavator,
		"getServerInfo":  GetServerInfo,
		"formatFileSize": FormatFileSize,
	})

	//从二进制中加载模板（后缀必须.html)
	templ := template.Must(template.New("").ParseFS(f, "template/html/*.html"))
	s.engine.SetHTMLTemplate(templ)
	//静态资源
	//s.engine.Static("/assets", "./template/assets")
	//s.engine.StaticFS("/public", http.FS(f))
	s.engine.GET("/", func(c *gin.Context) {
		s.GetclientIP(c)
		var v string
		ct.Invoke(func(version string) {
			v=version
		})
		c.HTML(http.StatusOK, "upcookie.html", gin.H{
			"version":v,
		})
	})
	// 静态文件处理
	s.engine.GET("assets/*action", func(c *gin.Context) {
		c.FileFromFS("template/assets/"+c.Param("action"),http.FS(f))
	})

	// 路由
	// 获取二维码
	s.engine.GET("/qrcode", s.getQrcode)
	// 获取返回的cookie信息
	s.engine.GET("/cookie", s.getCookie)
	// 获取各种配置文件api
	s.engine.GET("/api/config/:key")
	// 保存配置
	s.engine.POST("/api/upsave", s.upsave)
	s.engine.POST("/api/save")
	s.engine.GET("/home")
	s.engine.POST("/auth")
	//s.engine.GET("/test",s.test)

	// 初始化db
	s.initdb()

	go func() {
		log.Infof("jdcookie提取 服务器已启动: %v", addr)
		log.Info("请用浏览器打开url: http://公网ip或者域名%s",addr)
		log.Warn("请务必使用公网访问，否则读取到的客户端Ip会是内网Ip，不是公网Ip.")
		log.Warnf("v3.x 版本 是服务端部署版本。客户端需要使用浏览器打开，让浏览器和手机在同一个网络下（或者直接用手机打开浏览器）")
		s.HTTP = &http.Server{
			Addr:    addr,
			Handler: s.engine,
		}
		if err := s.HTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(err)
			log.Infof("HTTP 服务启动失败, 请检查端口是否被占用.")
			log.Warnf("将在五秒后退出.")
			time.Sleep(time.Second * 5)
			os.Exit(1)
		}
	}()
}

func (s *httpServer) initdb() {
	if s.Conf.DbConf.DbEnable {
		var err error
		err = repo.InitRDBMS(s.Conf.DbConf)
		if err != nil {
			log.Fatalf("faild to init db error= %s", err.Error())
		}
		s.cookiesRepo, err = repo.NewCookieRepo()
		if err != nil {
			log.Fatalf("faild to get initd db error= %s", err.Error())
		}
		s.cookiesRepo.InitTables()
	}
}
