package router

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/memstore"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/v03413/bepusdt/app/conf"
	"github.com/v03413/bepusdt/app/log"
	"github.com/v03413/bepusdt/app/model"
	"github.com/v03413/go-cache"
)

var engine *gin.Engine
var authRoute = make(map[string]bool)
var secureRoute = make(map[string]struct{})

func Handler() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	engine = gin.New()
	session := memstore.NewStore([]byte(model.GetK(model.AdminSecret)))
	session.Options(sessions.Options{
		MaxAge:   86400,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	})

	engine.Use(sessions.Sessions("session", session))
	engine.Use(gin.LoggerWithWriter(log.GetWriter()), gin.Recovery())
	engine.Use(sessionAuth(), copyright())
	engine.NoRoute(noRoute())
	engine.GET("/", func(ctx *gin.Context) {
		if !model.IsInstalled() {
			model.InstallLock()
			ctx.HTML(200, "installed.html", model.GetInstallInfo())

			return
		}

		sess := sessions.Default(ctx)
		if secure, ok := sess.Get(conf.AdminSecureK).(bool); ok && secure {
			ctx.HTML(200, "secure.html", gin.H{})
			return
		}

		if url := model.GetC(model.HomeRedirectUrl); url != "" {
			ctx.Redirect(302, url)
			return
		}

		ctx.HTML(200, "index.html", gin.H{"title": conf.Desc, "url": conf.Github})
	})

	{
		staticInit(engine)
		epusdtInit(engine)
		epayInit(engine)
		adminInit(engine)
		authInit(engine)
	}

	return engine
}

func sessionAuth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if conf.Debug {
			ctx.Next()
			return
		}

		var route = fmt.Sprintf("%s.%s", ctx.Request.Method, ctx.Request.URL.Path)
		if _, ok := secureRoute[route]; ok {
			sess := sessions.Default(ctx)
			if secure, ok := sess.Get(conf.AdminSecureK).(bool); !ok || !secure {
				ctx.JSON(403, gin.H{"code": 403, "msg": "unauthorized access"})
				ctx.Abort()
				return
			}
		}

		var need, ok = authRoute[route]
		if !ok || !need {
			ctx.Next()
			return
		}

		token, ok := cache.Get(conf.AdminTokenK)
		if !ok {
			ctx.JSON(403, gin.H{"code": 403, "msg": "token expired, please login again"})
			ctx.Abort()
			return
		}

		authHeader := ctx.GetHeader("Authorization")
		if authHeader == "" {
			ctx.JSON(403, gin.H{"code": 403, "msg": "missing authorization token"})
			ctx.Abort()
			return
		}

		if subtle.ConstantTimeCompare([]byte(cast.ToString(token)), []byte(authHeader)) != 1 {
			ctx.JSON(403, gin.H{"code": 403, "msg": "invalid authorization token"})
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}

func noRoute() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if ctx.Request.URL.Path == model.GetC(model.AdminSecure) {
			session := sessions.Default(ctx)
			session.Set(conf.AdminSecureK, true)
			_ = session.Save()

			ctx.Redirect(302, "/#/login")

			return
		}
	}
}

func copyright() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Writer.Header().Set("Payment-Gateway", "https://github.com/v03413/BEpusdt")
	}
}

// PayCors 收银台页面（如通过第三方 CDN 域名反代访问）需要跨域调用 /api/v1/pay/*
// 查询支付状态，此处按 CORS_ALLOWED_ORIGINS（逗号分隔）放行；未配置时默认放行任意来源，
// 因为这些接口本身面向匿名客户端，不含鉴权信息，安全性依赖 trade_id 的随机性而非同源限制。
func PayCors() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		origin := ctx.GetHeader("Origin")
		if origin != "" && isAllowedCorsOrigin(origin) {
			ctx.Header("Access-Control-Allow-Origin", origin)
			ctx.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			ctx.Header("Access-Control-Allow-Headers", "Content-Type")
			ctx.Header("Vary", "Origin")
		}

		if ctx.Request.Method == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusNoContent)

			return
		}

		ctx.Next()
	}
}

func isAllowedCorsOrigin(origin string) bool {
	var allowed = os.Getenv("CORS_ALLOWED_ORIGINS")
	if allowed == "" {

		return true
	}

	for _, o := range strings.Split(allowed, ",") {
		if strings.TrimSpace(o) == origin {

			return true
		}
	}

	return false
}

func PostRegister(router *gin.RouterGroup, relativePath string, checkAuth bool, handlers ...gin.HandlerFunc) {
	var route = fmt.Sprintf("POST.%s%s", router.BasePath(), relativePath)

	authRoute[route] = checkAuth
	secureRoute[route] = struct{}{}

	router.POST(relativePath, handlers...)
}

func GetRegister(router *gin.RouterGroup, relativePath string, checkAuth bool, handlers ...gin.HandlerFunc) {
	var route = fmt.Sprintf("GET.%s%s", router.BasePath(), relativePath)

	authRoute[route] = checkAuth
	secureRoute[route] = struct{}{}

	router.GET(relativePath, handlers...)
}
