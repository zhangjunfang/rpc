package goroutine

import (
	"sync/atomic"
	"time"
)

type workerWrapper struct {
	readyChan  chan int
	jobChan    chan interface{}
	outputChan chan interface{}
	poolOpen   uint32
	worker     GoroutineWorker
}

func (wrapper *workerWrapper) Loop() {

	// TODO: Configure?
	tout := time.Duration(5)
	for !wrapper.worker.Ready() {
		// It's sad that we can't simply check if jobChan is closed here.
		if atomic.LoadUint32(&wrapper.poolOpen) == 0 {
			break
		}
		time.Sleep(tout * time.Millisecond)
	}

	wrapper.readyChan <- 1

	for data := range wrapper.jobChan {
		wrapper.outputChan <- wrapper.worker.Job(data)
		for !wrapper.worker.Ready() {
			if atomic.LoadUint32(&wrapper.poolOpen) == 0 {
				break
			}
			time.Sleep(tout * time.Millisecond)
		}
		wrapper.readyChan <- 1
	}

	close(wrapper.readyChan)
	close(wrapper.outputChan)

}

func (wrapper *workerWrapper) Open() {
	if extWorker, ok := wrapper.worker.(GoroutineExtendedWorker); ok {
		extWorker.Initialize()
	}

	wrapper.readyChan = make(chan int)
	wrapper.jobChan = make(chan interface{})
	wrapper.outputChan = make(chan interface{})

	atomic.SwapUint32(&wrapper.poolOpen, uint32(1))

	go wrapper.Loop()
}

// Follow this with Join(), otherwise terminate isn't called on the worker
func (wrapper *workerWrapper) Close() {
	close(wrapper.jobChan)

	// Breaks the worker out of a Ready() -> false loop
	atomic.SwapUint32(&wrapper.poolOpen, uint32(0))
}

func (wrapper *workerWrapper) Join() {
	// Ensure that both the ready and output channels are closed
	for {
		_, readyOpen := <-wrapper.readyChan
		_, outputOpen := <-wrapper.outputChan
		if !readyOpen && !outputOpen {
			break
		}
	}

	if extWorker, ok := wrapper.worker.(GoroutineExtendedWorker); ok {
		extWorker.Terminate()
	}
}

func (wrapper *workerWrapper) Interrupt() {
	if extWorker, ok := wrapper.worker.(GoroutineInterruptable); ok {
		extWorker.Interrupt()
	}
}
