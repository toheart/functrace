package trace

import (
	"time"

	"github.com/sirupsen/logrus"
	"github.com/toheart/functrace/domain/model"
)

// InitGoroutineIfNeeded 检查goroutine是否已在跟踪中，如果不在则初始化
func (t *TraceInstance) InitGoroutineIfNeeded(gid uint64, name string) (info *GoroutineInfo, initFunc bool) {
	// 判断该goroutine是否已经被跟踪
	t.RLock()
	info, exists := t.GoroutineRunning[gid]
	t.RUnlock()

	if exists {
		return info, false
	}

	t.Lock()
	defer t.Unlock()
	// 二次检查
	info, exists = t.GoroutineRunning[gid]
	if exists {
		return info, false
	}
	// 更新运行中的goroutine映射
	start := time.Now()
	id := t.gGroutineId.Add(1)
	info = &GoroutineInfo{
		ID:             id,
		OriginGID:      gid,
		LastUpdateTime: start.Format(TimeFormat),
	}
	t.GoroutineRunning[gid] = info

	t.sendOp(&DataOp{
		OpType: OpTypeInsert,
		Arg: &model.GoroutineTrace{
			ID:           int64(id),
			OriginGID:    gid,
			CreateTime:   start.Format(TimeFormat),
			IsFinished:   0,
			InitFuncName: name,
		},
	})

	t.log.WithFields(logrus.Fields{"goroutine": id, "initFunc": name}).Info("initialized goroutine trace")

	return info, true
}

// SetGoroutineRunning 更新协程运行状态
func (t *TraceInstance) SetGoroutineRunning(info *GoroutineInfo) {
	t.Lock()
	t.GoroutineRunning[info.OriginGID] = info
	t.Unlock()
}

// GoroutineFinished 标记协程已完成
// Deprecated: 请使用 finishGoroutineTrace 方法替代
func (t *TraceInstance) GoroutineFinished(info *GoroutineInfo) {
	t.finishGoroutineTrace(info)
}

// finishGoroutineTrace 完成对goroutine的跟踪
func (t *TraceInstance) finishGoroutineTrace(info *GoroutineInfo) {
	t.log.WithFields(logrus.Fields{"id": info.ID}).Info("finishing goroutine trace")

	// 获取协程信息
	goroutine, err := repositoryFactory.GetGoroutineRepository().FindGoroutineByID(int64(info.ID))
	if err != nil {
		t.log.WithFields(logrus.Fields{"error": err}).Error("get goroutine info failed")
		return
	}

	// 解析创建时间
	createTime, err := time.Parse(TimeFormat, goroutine.CreateTime)
	if err != nil {
		t.log.WithFields(logrus.Fields{"error": err}).Error("parse create time failed")
		// 在解析失败的情况下，使用默认的时间成本
		t.sendFinishedGoroutineOp(goroutine, time.Since(currentNow).String())
		return
	}

	// 计算总运行时间
	totalExecTime := time.Since(createTime)

	// 更新协程数据
	t.sendFinishedGoroutineOp(goroutine, totalExecTime.String())

	// 从映射中移除
	t.deleteGoroutineRunning(info.OriginGID)
	// 同时移除会话
	if t.sessions != nil {
		// 优雅关闭会话，确保数据转发完成
		s := t.sessions.GetOrCreate(info.OriginGID)
		s.Close()
		t.sessions.Remove(info.OriginGID)
	}
	t.log.WithFields(logrus.Fields{
		"goroutine identifier": info.OriginGID,
		"database id":          info.ID,
		"total execution time": totalExecTime.String(),
	}).Info("completed goroutine trace")
}

// deleteGoroutineRunning 从映射中移除goroutine
func (t *TraceInstance) deleteGoroutineRunning(gid uint64) {
	t.Lock()
	delete(t.GoroutineRunning, gid)
	t.Unlock()
}

// monitorGoroutines 定时监控goroutine的运行状态
func (t *TraceInstance) monitorGoroutines() {
	// 获取监控间隔时间
	interval := t.config.MonitorInterval

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.checkAndFinishGoroutines()
		case <-stopMonitor:
			t.log.Info("goroutine monitor stopped")
			return
		}
	}
}

// GetGoroutineRunning 获取当前运行中的goroutine映射
func (t *TraceInstance) GetGoroutineRunning() map[uint64]*GoroutineInfo {
	t.RLock()
	defer t.RUnlock()
	runningGoroutines := make(map[uint64]*GoroutineInfo)
	for k, v := range t.GoroutineRunning {
		runningGoroutines[k] = v
	}
	return runningGoroutines
}

// checkAndFinishGoroutines 检查并完成已结束的goroutine跟踪
func (t *TraceInstance) checkAndFinishGoroutines() {
	t.log.Info("checking and finishing finished goroutine traces")
	// 获取当前运行的所有goroutine ID
	currentGoroutines := make(map[uint64]bool)
	for _, id := range t.getAllGoroutineIDs() {
		currentGoroutines[uint64(id)] = true
	}

	// 复制当前运行中的goroutine映射，以避免在迭代过程中修改
	var finishedGoroutines []*GoroutineInfo

	runningGoroutines := t.GetGoroutineRunning()

	// 检查哪些goroutine已经结束但未更新
	for _, info := range runningGoroutines {
		if _, exists := currentGoroutines[info.OriginGID]; !exists {
			// 该goroutine不再运行，需要完成其跟踪
			finishedGoroutines = append(finishedGoroutines, info)
		}
	}
	t.log.Info("finished goroutines", "count", len(finishedGoroutines))
	// 处理已结束的goroutine
	for _, info := range finishedGoroutines {
		t.finishGoroutineTrace(info)
	}

	t.log.Info("goroutine monitor check completed",
		"running count", len(runningGoroutines),
		"finished count", len(finishedGoroutines))
}

// SetGoroutineStarted 设置协程已启动
func (t *TraceInstance) SetGoroutineStarted(gid uint64, originGid uint64, funcName string) {
	goroutine := &model.GoroutineTrace{
		OriginGID:    originGid,
		CreateTime:   time.Now().Format(TimeFormat),
		IsFinished:   0,
		InitFuncName: funcName,
	}

	// 使用持久化层保存协程数据
	go func() {
		id, err := repositoryFactory.GetGoroutineRepository().SaveGoroutine(goroutine)
		if err != nil {
			t.log.WithFields(logrus.Fields{
				"error":     err,
				"goroutine": goroutine,
			}).Error("保存协程数据失败")
			return
		}

		// 更新 GoroutineRunning 映射
		t.Lock()
		t.GoroutineRunning[gid] = &GoroutineInfo{
			ID:             uint64(id), // 将 int64 转换为 uint64
			OriginGID:      originGid,
			LastUpdateTime: time.Now().Format(TimeFormat),
		}
		t.Unlock()
	}()
}

// SetGoroutineFinished 设置协程已完成
// Deprecated: 请使用 finishGoroutineTrace 方法替代
func (t *TraceInstance) SetGoroutineFinished(gid uint64, info *GoroutineInfo) {
	t.finishGoroutineTrace(info)
}

func (t *TraceInstance) saveGoroutineTrace(goroutine *model.GoroutineTrace) {
	_, err := repositoryFactory.GetGoroutineRepository().SaveGoroutine(goroutine)
	if err != nil {
		t.log.WithFields(logrus.Fields{
			"error":     err,
			"goroutine": goroutine,
		}).Error("保存协程数据失败")
	}
}

// updateGoroutineTimeCost 更新goroutine时间成本
func (t *TraceInstance) updateGoroutineTimeCost(goroutine *model.GoroutineTrace) {
	t.log.WithFields(logrus.Fields{"id": goroutine.ID, "timeCost": goroutine.TimeCost, "isFinished": goroutine.IsFinished}).Info("updating goroutine trace with time cost")

	// 使用持久化层更新协程时间成本
	err := repositoryFactory.GetGoroutineRepository().UpdateGoroutineTimeCost(int64(goroutine.ID), goroutine.TimeCost, goroutine.IsFinished)
	if err != nil {
		t.log.WithFields(logrus.Fields{
			"error":      err,
			"id":         goroutine.ID,
			"timeCost":   goroutine.TimeCost,
			"isFinished": goroutine.IsFinished,
		}).Error("Failed to update goroutine time cost")
	}
}

func (t *TraceInstance) sendFinishedGoroutineOp(goroutine *model.GoroutineTrace, timeCost string) {
	goroutine.TimeCost = timeCost
	goroutine.IsFinished = 1
	t.sendOp(&DataOp{
		OpType: OpTypeUpdate,
		Arg:    goroutine,
	})
}
