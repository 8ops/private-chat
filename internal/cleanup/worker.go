package cleanup

import (
	"os"
	"time"

	"private-chat/internal/logger"
	"private-chat/internal/repo"
)

// Worker 周期性清理过期消息与关联文件。
type Worker struct {
	messages *repo.MessageRepo
	files    *repo.FileRepo
	tasks    *repo.CleanupTaskRepo
	retention time.Duration
	interval  time.Duration
	maxRetry  int
	stop      chan struct{}
}

// New 创建清理 Worker。
func New(messages *repo.MessageRepo, files *repo.FileRepo, tasks *repo.CleanupTaskRepo, retention time.Duration) *Worker {
	return &Worker{
		messages:  messages,
		files:     files,
		tasks:     tasks,
		retention: retention,
		interval:  time.Minute,
		maxRetry:  10,
		stop:      make(chan struct{}),
	}
}

// Start 启动后台循环。
func (w *Worker) Start() {
	go w.run()
	logger.Info("cleanup worker started", map[string]interface{}{
		"interval":  w.interval.String(),
		"retention": w.retention.String(),
	})
}

// Stop 停止循环。
func (w *Worker) Stop() { close(w.stop) }

func (w *Worker) run() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			w.cycle()
		}
	}
}

func (w *Worker) cycle() {
	before := time.Now().Add(-w.retention)

	// 1) 收集可删除的过期文件（仅被过期消息引用的文件）。
	fileIDs, err := w.messages.CollectExpiredFileIDs(before)
	if err != nil {
		logger.Error("cleanup collect file ids failed", map[string]interface{}{"error": err.Error()})
	} else if len(fileIDs) > 0 {
		for _, fid := range fileIDs {
			w.deleteFile(fid)
		}
	}

	// 2) 删除过期消息。
	expired, err := w.messages.GetExpired(before, 1000)
	if err != nil {
		logger.Error("cleanup get expired messages failed", map[string]interface{}{"error": err.Error()})
		return
	}
	for _, m := range expired {
		if err := w.messages.Delete(m.ID); err != nil {
			logger.Error("cleanup delete message failed", map[string]interface{}{"error": err.Error(), "id": m.ID})
		} else {
			logger.Info("message deleted", map[string]interface{}{"id": m.ID, "type": m.MessageType})
		}
	}

	// 3) 重试之前失败的文件删除任务。
	w.retryFailed()
}

// deleteFile 删除文件记录与物理文件，失败则写入重试任务。
func (w *Worker) deleteFile(fileID string) {
	f, err := w.files.GetByID(fileID)
	if err != nil {
		// 记录不存在，视为已清理。
		return
	}
	if f.DeletedAt != nil {
		return
	}
	path := f.Path
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logger.Error("cleanup file remove failed", map[string]interface{}{"error": err.Error(), "path": path})
		if werr := w.tasks.UpsertFailed(fileID, err.Error()); werr != nil {
			logger.Error("cleanup upsert task failed", map[string]interface{}{"error": werr.Error()})
		}
		return
	}
	if err := w.files.MarkDeleted(fileID); err != nil {
		logger.Error("cleanup mark file deleted failed", map[string]interface{}{"error": err.Error()})
	}
	logger.Info("file deleted", map[string]interface{}{"id": fileID, "path": path})
}

func (w *Worker) retryFailed() {
	pending, err := w.tasks.ListPending(w.maxRetry)
	if err != nil {
		logger.Error("cleanup list pending failed", map[string]interface{}{"error": err.Error()})
		return
	}
	for _, fid := range pending {
		f, err := w.files.GetByID(fid)
		if err != nil {
			_ = w.tasks.MarkDone(fid)
			continue
		}
		if err := os.Remove(f.Path); err != nil && !os.IsNotExist(err) {
			_ = w.tasks.UpsertFailed(fid, err.Error())
			continue
		}
		_ = w.files.MarkDeleted(fid)
		_ = w.tasks.MarkDone(fid)
		logger.Info("cleanup retry succeeded", map[string]interface{}{"id": fid})
	}
}
