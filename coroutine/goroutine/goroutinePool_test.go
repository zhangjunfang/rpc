package goroutine

import (
	"sync"
	"testing"
	"time"
)

func TestBasicJob(t *testing.T) {
	pool, err := CreatePool(1, func(in interface{}) interface{} {
		intVal := in.(int)
		return intVal * 2
	}).Open()
	if err != nil {
		t.Errorf("Failed to create pool: %v", err)
		return
	}
	defer pool.Close()

	for i := 0; i < 1; i++ {
		ret, err := pool.SendWork(10)
		if err != nil {
			t.Errorf("Failed to send work: %v", err)
			return
		}
		retInt := ret.(int)
		if ret != 20 {
			t.Errorf("Wrong return value: %v != %v", 20, retInt)
		}
	}
}

func TestParallelJobs(t *testing.T) {
	nWorkers := 10

	jobGroup := sync.WaitGroup{}
	testGroup := sync.WaitGroup{}

	pool, err := CreatePool(nWorkers, func(in interface{}) interface{} {
		jobGroup.Done()
		jobGroup.Wait()

		intVal := in.(int)
		return intVal * 2
	}).Open()
	if err != nil {
		t.Errorf("Failed to create pool: %v", err)
		return
	}
	defer pool.Close()

	for j := 0; j < 1; j++ {
		jobGroup.Add(nWorkers)
		testGroup.Add(nWorkers)

		for i := 0; i < nWorkers; i++ {
			go func() {
				ret, err := pool.SendWork(10)
				if err != nil {
					t.Errorf("Failed to send work: %v", err)
					return
				}
				retInt := ret.(int)
				if ret != 20 {
					t.Errorf("Wrong return value: %v != %v", 20, retInt)
				}

				testGroup.Done()
			}()
		}

		testGroup.Wait()
	}
}

/*--------------------------------------------------------------------------------------------------
 */

// Basic worker implementation
type dummyWorker struct {
	ready bool
	t     *testing.T
}

func (d *dummyWorker) Job(in interface{}) interface{} {
	if !d.ready {
		d.t.Errorf("TunnyJob called without polling TunnyReady")
	}
	d.ready = false
	return in
}

func (d *dummyWorker) Ready() bool {
	d.ready = true
	return d.ready
}

// Test the pool with a basic worker implementation
func TestDummyWorker(t *testing.T) {
	pool, err := CreateCustomPool([]GoroutineWorker{&dummyWorker{t: t}}).Open()
	if err != nil {
		t.Errorf("Failed to create pool: %v", err)
		return
	}
	defer pool.Close()

	for i := 0; i < 100; i++ {
		if result, err := pool.SendWork(12); err != nil {
			t.Errorf("Failed to send work: %v", err)
		} else if resInt, ok := result.(int); !ok || resInt != 12 {
			t.Errorf("Unexpected result from job: %v != %v", 12, result)
		}
	}
}

// Extended worker implementation
type dummyExtWorker struct {
	dummyWorker

	initialized bool
}

func (d *dummyExtWorker) TunnyJob(in interface{}) interface{} {
	if !d.initialized {
		d.t.Errorf("TunnyJob called without calling TunnyInitialize")
	}
	return d.dummyWorker.Job(in)
}

func (d *dummyExtWorker) Initialize() {
	d.initialized = true
}

func (d *dummyExtWorker) Terminate() {
	if !d.initialized {
		d.t.Errorf("TunnyTerminate called without calling TunnyInitialize")
	}
	d.initialized = false
}

// Test the pool with an extended worker implementation
func TestDummyExtWorker(t *testing.T) {
	pool, err := CreateCustomPool(
		[]GoroutineWorker{
			&dummyExtWorker{
				dummyWorker: dummyWorker{t: t},
			},
		}).Open()
	if err != nil {
		t.Errorf("Failed to create pool: %v", err)
		return
	}
	defer pool.Close()

	for i := 0; i < 100; i++ {
		if result, err := pool.SendWork(12); err != nil {
			t.Errorf("Failed to send work: %v", err)
		} else if resInt, ok := result.(int); !ok || resInt != 12 {
			t.Errorf("Unexpected result from job: %v != %v", 12, result)
		}
	}
}

// Extended and interruptible worker implementation
type dummyExtIntWorker struct {
	dummyExtWorker

	jobLock *sync.Mutex
}

func (d *dummyExtIntWorker) Job(in interface{}) interface{} {
	d.jobLock.Lock()
	d.jobLock.Unlock()

	return d.dummyExtWorker.TunnyJob(in)
}

func (d *dummyExtIntWorker) Ready() bool {
	d.jobLock.Lock()

	return d.dummyExtWorker.Ready()
}

func (d *dummyExtIntWorker) Interrupt() {
	d.jobLock.Unlock()
}

// Test the pool with an extended and interruptible worker implementation
func TestDummyExtIntWorker(t *testing.T) {
	pool, err := CreateCustomPool(
		[]GoroutineWorker{
			&dummyExtIntWorker{
				dummyExtWorker: dummyExtWorker{
					dummyWorker: dummyWorker{t: t},
				},
				jobLock: &sync.Mutex{},
			},
		}).Open()
	if err != nil {
		t.Errorf("Failed to create pool: %v", err)
		return
	}
	defer pool.Close()

	for i := 0; i < 100; i++ {
		if _, err := pool.SendWorkTimed(1, nil); err == nil {
			t.Errorf("Expected timeout from dummyExtIntWorker.")
		}
	}
}

func TestNumWorkers(t *testing.T) {
	numWorkers := 10
	pool, err := CreatePoolGeneric(numWorkers).Open()
	if err != nil {
		t.Errorf("Failed to create pool: %v", err)
		return
	}
	defer pool.Close()
	actual := pool.NumWorkers()
	if actual != numWorkers {
		t.Errorf("Expected to get %d workers, but got %d", numWorkers, actual)
	}
}

var waitHalfSecond = func() {
	time.Sleep(500 * time.Millisecond)
}

func TestNumPendingReportsAllWorkersWithNoWork(t *testing.T) {
	numWorkers := 10
	pool, err := CreatePoolGeneric(numWorkers).Open()
	if err != nil {
		t.Errorf("Failed to create pool: %v", err)
		return
	}
	defer pool.Close()
	actual := pool.NumPendingAsyncJobs()
	if actual != 0 {
		t.Errorf("Expected to get 0 pending jobs when pool is quiet, but got %d", actual)
	}
}

func TestNumPendingReportsNotAllWorkersWhenSomeBusy(t *testing.T) {
	numWorkers := 10
	pool, err := CreatePoolGeneric(numWorkers).Open()
	if err != nil {
		t.Errorf("Failed to create pool: %v", err)
		return
	}
	defer pool.Close()
	pool.SendWorkAsync(waitHalfSecond, nil)
	actual := pool.NumPendingAsyncJobs()
	expected := int32(1)
	if actual != expected {
		t.Errorf("Expected to get %d pending jobs when pool has work, but got %d", expected, actual)
	}
}
