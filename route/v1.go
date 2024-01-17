package route

import (
	"crypto/ecdsa"
	"os"

	"github.com/IceWhaleTech/CasaOS-Common/external"
	"github.com/IceWhaleTech/CasaOS-Common/middleware"
	"github.com/IceWhaleTech/CasaOS-Common/utils/jwt"
	"github.com/IceWhaleTech/CasaOS-LocalStorage/pkg/config"
	v1 "github.com/IceWhaleTech/CasaOS-LocalStorage/route/v1"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func InitV1Router() *gin.Engine {
	// check if environment variable is set
	ginMode, success := os.LookupEnv(gin.EnvGinMode)
	if !success {
		ginMode = gin.ReleaseMode
	}
	gin.SetMode(ginMode)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Cors())
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	if ginMode != gin.ReleaseMode {
		r.Use(middleware.WriteLog())
	}
	r.GET("/v1/recover/:type", v1.GetRecoverStorage)
	v1Group := r.Group("/v1")

	v1Group.Use(jwt.JWT(
		func() (*ecdsa.PublicKey, error) {
			return external.GetPublicKey(config.CommonInfo.RuntimePath)
		},
	)) // jwt验证

	{
		v1DisksGroup := v1Group.Group("/disks")
		v1DisksGroup.Use()
		{

			v1DisksGroup.GET("", v1.GetDiskList)          // 获取磁盘列表
			v1DisksGroup.GET("/usb", v1.GetDisksUSBList)  // 获取USB磁盘列表
			v1DisksGroup.DELETE("/usb", v1.DeleteDiskUSB) // 删除USB磁盘
			v1DisksGroup.DELETE("", v1.DeleteDisksUmount) // 卸载磁盘
			v1DisksGroup.GET("/size", v1.GetDiskSize)     // 获取磁盘大小
		}

		v1StorageGroup := v1Group.Group("/storage")
		v1StorageGroup.Use()
		{
			v1StorageGroup.POST("", v1.PostAddStorage) // 添加存储

			v1StorageGroup.PUT("", v1.PutFormatStorage) // 格式化存储

			v1StorageGroup.DELETE("", v1.DeleteStorage) // 删除存储
			v1StorageGroup.GET("", v1.GetStorageList)   // 获取存储列表
		}
		v1CloudGroup := v1Group.Group("/cloud")
		v1CloudGroup.Use()
		{
			v1CloudGroup.GET("", v1.ListStorages)     // 获取云盘列表
			v1CloudGroup.DELETE("", v1.UmountStorage) // 卸载云盘
		}
		v1DriverGroup := v1Group.Group("/driver")
		v1DriverGroup.Use()
		{
			v1DriverGroup.GET("", v1.ListDriverInfo) // 获取驱动信息
		}
		v1USBGroup := v1Group.Group("/usb")
		v1USBGroup.Use()
		{
			v1USBGroup.PUT("/usb-auto-mount", v1.PutSystemUSBAutoMount) ///sys/usb/:status //设置USB自动挂载
			v1USBGroup.GET("/usb-auto-mount", v1.GetSystemUSBAutoMount) ///sys/usb/status //获取USB自动挂载状态
		}
	}

	return r
}
