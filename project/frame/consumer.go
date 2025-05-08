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

	waitAll.Add(1)
	atomic.AddUint32(&goUid, 1)
	atomic.AddInt32(&goCount, 1)

	go func() {
		// 优先执行当前
		TryIt(fn, nil)
		// 加入等待队列
		for pc <= poolSize {
			select {
			case <-stopChanForGo:
				pc = poolSize + 1
			case fun := <-poolChan:
				TryIt(fun, nil)
			}
		}
		// 生命周期
		waitAll.Done()
		atomic.AddInt32(&goCount, -1)
	}()
}

func TryIt(fun func(), catch func(interface{})) {
	defer func() {
		if err := recover(); err != nil {
			if catch != nil {
				catch(err)
			}
		}
	}()
	fun()
}
