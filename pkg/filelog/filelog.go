package filelog

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type FileLog struct {
	AppName       string
	FilePath      string
	Date          time.Time
	enableAtomic  int32
	writeCh       chan []byte
	ctx           context.Context
	cancel        context.CancelFunc
	wgWriteWorker sync.WaitGroup
}

func New(appName string, bufferSize int) *FileLog {
	ctx, cancel := context.WithCancel(context.Background())
	fl := &FileLog{
		AppName: appName,
		Date:    time.Now(),
		writeCh: make(chan []byte, bufferSize),
		ctx:     ctx,
		cancel:  cancel,
	}
	fl.SetEnable(false)
	fl.wgWriteWorker.Add(1)
	go fl.writeWorker()
	return fl
}

func (fl *FileLog) writeWorker() {
	var f *os.File
	for {
		select {
		case p := <-fl.writeCh:
			f = fl.ensureFileOpen(f)
			if f != nil {
				_, _ = f.Write(p)
			}
		case <-fl.ctx.Done():
			close(fl.writeCh)
			// Drain remaining data in writeCh
			for p := range fl.writeCh {
				f = fl.ensureFileOpen(f)
				if f != nil {
					_, _ = f.Write(p)
				}
			}
			if f != nil {
				_ = f.Close()
			}
			fl.wgWriteWorker.Done()
			return
		}
	}
}

func (fl *FileLog) ensureFileOpen(f *os.File) *os.File {
	if f == nil {
		var err error
		filename, err := fl.createFilePath()
		if err != nil {
			return nil
		}
		fl.FilePath = filename
		f, err = fl.openFile(filename)
		if err != nil {
			return nil
		}
	}
	return f
}

// openFile opens the log file for read-write, creating it and any necessary directories if it doesn't exist.
func (fl *FileLog) openFile(filename string) (*os.File, error) {
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0665)
	if err != nil {
		if err = os.MkdirAll(filepath.Dir(filename), 0766); err != nil {
			return nil, err
		}
		f, err = os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	}
	return f, err
}

func (fl *FileLog) Close() {
	fl.cancel()
	fl.wgWriteWorker.Wait()
}

// createFilePath generates the file path for the log file based on the current date.
func (fl *FileLog) createFilePath() (string, error) {
	confDir, err := os.UserConfigDir()
	if err != nil {
		return confDir, err
	}
	year := fl.Date.Format("2006")
	monthDate := fl.Date.Format("01-02")
	return filepath.Join(confDir, fl.AppName, year, monthDate+".log"), nil
}

// Write writes the provided byte slice to the log file, creating the file if necessary.
func (fl *FileLog) Write(p []byte) (n int, err error) {
	if !fl.GetEnable() {
		return os.Stderr.Write(p)
	}
	select {
	case fl.writeCh <- p:
		return len(p), nil
	case <-fl.ctx.Done():
		return 0, fl.ctx.Err()
	}
}

// SetEnable sets the fl.Enable flag in a thread-safe manner using atomic operations.
func (fl *FileLog) SetEnable(enable bool) {
	var enableAtomic int32
	if enable {
		enableAtomic = 1
	}
	atomic.StoreInt32(&fl.enableAtomic, enableAtomic)
}

// GetEnable returns the current value of the fl.Enable flag in a thread-safe manner using atomic operations.
func (fl *FileLog) GetEnable() bool {
	return atomic.LoadInt32(&fl.enableAtomic) != 0
}
