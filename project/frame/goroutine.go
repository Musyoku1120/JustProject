package frame

import "sync/atomic"

func Gogo(fn func()) {
	pc := poolSize + 1
	select {
	case poolChan <- fn:
		return
	default:
		pc = atomic.AddInt32(&poolGoCount, 1)
		if pc > poolSize {
			atomic.AddInt32(&poolGoCount, -1)
		}
	}

	waitAllGo.Add(1)
	atomic.AddUint32(&goUid, 1)
	atomic.AddInt32(&goCount, 1)

	go func() {
		defer waitAllGo.Done()
		// 优先执行当前
		TryIt(fn, nil) // 循环任务会导致wait阻塞
		// 加入等待队列
		for pc <= poolSize {
			select {
			case <-stopChForGo:
				pc = poolSize + 1
			case fun := <-poolChan: // 无缓冲区通道 进入case后陷入阻塞
				TryIt(fun, nil)
			}
		}
		// 生命周期
		cnt := atomic.AddInt32(&goCount, -1)
		LogDebug("goroutine: %v exit", cnt+1)
	}()
}

func TryIt(fun func(), catch func(interface{})) {
	defer func() {
		if err := recover(); err != nil {
			LogPanic()
			if catch != nil {
				catch(err)
			}
		}
	}()
	fun()
}

func systemGo(fn func(stopCh chan struct{})) {
	waitAllSys.Add(1)
	go func() {
		fn(stopChForSys)
		waitAllSys.Done()
	}()
}
