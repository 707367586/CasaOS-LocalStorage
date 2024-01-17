package main

import (
	"context"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/IceWhaleTech/CasaOS-LocalStorage/common"
	"github.com/IceWhaleTech/CasaOS-LocalStorage/model"
	"github.com/IceWhaleTech/CasaOS-LocalStorage/service"
	"github.com/pilebones/go-udev/netlink"
	"go.uber.org/zap"
)

func sendDiskBySocket() {
	blkList := service.MyService.Disk().LSBLK(true)

	status := model.DiskStatus{}
	healthy := true

	// var systemDisk *model.LSBLKModel

	for _, currentDisk := range blkList {

		// if systemDisk == nil {
		// 	// go 5 level deep to look for system block device by mount point being "/"
		// 	systemDisk = service.WalkDisk(currentDisk, 5, func(blk model.LSBLKModel) bool { return blk.MountPoint == "/" })

		// 	if systemDisk != nil {
		// 		s, _ := strconv.ParseUint(systemDisk.FSSize.String(), 10, 64)
		// 		a, _ := strconv.ParseUint(systemDisk.FSAvail.String(), 10, 64)
		// 		u, _ := strconv.ParseUint(systemDisk.FSUsed.String(), 10, 64)
		// 		status.Size += s
		// 		status.Avail += a
		// 		status.Used += u
		//		continue
		// 	}
		// }
		if !service.IsDiskSupported(currentDisk) {
			continue
		}
		temp := service.MyService.Disk().SmartCTL(currentDisk.Path)
		if reflect.DeepEqual(temp, model.SmartctlA{}) {
			healthy = true
		} else {
			if len(temp.ModelName) > 0 {
				healthy = temp.SmartStatus.Passed
			} else {
				healthy = true
			}
		}
		if len(currentDisk.Children) > 0 {
			for _, v := range currentDisk.Children {
				if len(v.MountPoint) > 0 {
					s, _ := strconv.ParseUint(v.FSSize.String(), 10, 64)
					a, _ := strconv.ParseUint(v.FSAvail.String(), 10, 64)
					u, _ := strconv.ParseUint(v.FSUsed.String(), 10, 64)
					status.Size += s
					status.Avail += a
					status.Used += u
				}
			}
		} else {
			if len(currentDisk.MountPoint) > 0 {
				s, _ := strconv.ParseUint(currentDisk.FSSize.String(), 10, 64)
				a, _ := strconv.ParseUint(currentDisk.FSAvail.String(), 10, 64)
				u, _ := strconv.ParseUint(currentDisk.FSUsed.String(), 10, 64)
				status.Size += s
				status.Avail += a
				status.Used += u
			}
		}
	}

	status.Health = healthy
	message := make(map[string]interface{})
	message["sys_disk"] = status
	if err := service.MyService.NotifySystem().SendSystemStatusNotify(message); err != nil {
		logger.Error("failed to send notify", zap.Any("message", message), zap.Error(err))
	}
}

func sendUSBBySocket() {
	message := map[string]interface{}{
		"sys_usb": service.MyService.Disk().GetUSBDriveStatusList(),
	}

	if err := service.MyService.NotifySystem().SendSystemStatusNotify(message); err != nil {
		logger.Error("failed to send notify", zap.Any("message", message), zap.Error(err))
	}
}

func monitorUEvent(ctx context.Context) {
	var matcher netlink.Matcher

	conn := new(netlink.UEventConn) // 创建 Netlink UEvent 连接
	if err := conn.Connect(netlink.UdevEvent); err != nil {
		logger.Error("udev err", zap.Any("Unable to connect to Netlink Kobject UEvent socket", err))
	}
	defer conn.Close()

	queue := make(chan netlink.UEvent) // 创建用于接收 UEvent 的通道
	defer close(queue)

	errors := make(chan error) // 创建用于报告错误的通道
	defer close(errors)

	quit := conn.Monitor(queue, errors, matcher) // 启动 UEvent 监听
	defer close(quit)
	// 无限循环监听事件
	for {
		select {

		case <-ctx.Done(): // 当上下文被取消时退出循环
			return

		case uevent := <-queue:

			if event := common.EventAdapter(uevent); event != nil {

				// add UI properties to applicable events so that CasaOS UI can render it
				event := common.EventAdapterWithUIProperties(event)

				if v, ok := event.Properties["local-storage:path"]; ok && strings.Contains(event.Name, "disk") {

					diskModel := service.MyService.Disk().GetDiskInfo(v)
					if !reflect.DeepEqual(diskModel, model.LSBLKModel{}) {

						properties := common.AdditionalProperties(diskModel)
						for k, v := range properties {
							event.Properties[k] = v
						}
					}
				}
				logger.Info("disk model", zap.Any("diskModel", event.Name))
				// 发送事件到消息总线
				response, err := service.MyService.MessageBus().PublishEventWithResponse(ctx, event.SourceID, event.Name, event.Properties)
				if err != nil {
					logger.Error("failed to publish event to message bus", zap.Error(err), zap.Any("event", event))
				}
				// 处理消息总线的响应
				if response.StatusCode() != http.StatusOK {
					logger.Error("failed to publish event to message bus", zap.String("status", response.Status()), zap.Any("response", response))
				}
			}

			switch uevent.Env["DEVTYPE"] {
			case "partition":

				switch uevent.Env["ID_BUS"] {
				case "usb":
					time.Sleep(1 * time.Second)
					sendUSBBySocket()
					continue
				}
			}

		case err := <-errors:
			logger.Error("udev err", zap.Error(err))
		}
	}
}

func sendStorageStats() {
	sendDiskBySocket() // 发送磁盘状态给casaos
	sendUSBBySocket()  // 发送usb状态给casaos
}
