package stream

import (
	"context"
	"strings"
	"sync"
	"time"
)

type StringAccumulator struct {
	maxLength    int
	timeout      time.Duration
	inputChan    <-chan string
	processFunc  func(context.Context, string)
	buffer       strings.Builder
	lastCallTime time.Time
	mutex        sync.RWMutex
	wg           sync.WaitGroup
}

func NewStringAccumulator(inputChan <-chan string, maxLength int, timeout time.Duration, processFunc func(context.Context, string)) *StringAccumulator {
	if processFunc == nil {
		panic("processFunc must not be nil")
	}

	return &StringAccumulator{
		maxLength:    maxLength,
		timeout:      timeout,
		inputChan:    inputChan,
		processFunc:  processFunc,
		lastCallTime: time.Now(),
	}
}

func (sa *StringAccumulator) Start(ctx context.Context) {
	sa.wg.Add(1)
	go sa.processLoop(ctx)
}

func (sa *StringAccumulator) processLoop(ctx context.Context) {
	defer sa.wg.Done()

	ticker := time.NewTicker(sa.timeout)
	defer ticker.Stop()

	defer sa.flush()

	for {
		select {
		case <-ctx.Done():
			return

		case str, ok := <-sa.inputChan:
			if !ok {
				return
			}

			sa.accumulateAndProcess(ctx, str)

		case <-ticker.C:
			sa.processIfTimeout(ctx)
		}
	}
}

func (sa *StringAccumulator) accumulateAndProcess(ctx context.Context, str string) {
	sa.mutex.Lock()
	defer sa.mutex.Unlock()

	sa.buffer.WriteString(" ")
	sa.buffer.WriteString(str)

	if sa.buffer.Len() < sa.maxLength {
		return
	}

	sa.processBuffer(ctx)
}

func (sa *StringAccumulator) processIfTimeout(ctx context.Context) {
	sa.mutex.Lock()
	defer sa.mutex.Unlock()

	if time.Since(sa.lastCallTime) < sa.timeout || sa.buffer.Len() == 0 {
		return
	}

	sa.processBuffer(ctx)
}

func (sa *StringAccumulator) processBuffer(ctx context.Context) {
	if sa.buffer.Len() == 0 {
		return
	}

	content := strings.TrimSpace(sa.buffer.String())

	sa.buffer.Reset()
	sa.lastCallTime = time.Now()

	if content != "" {
		sa.processFunc(ctx, content)
	}
}

func (sa *StringAccumulator) flush() {
	tempCtx, cancel := context.WithTimeout(context.Background(), sa.timeout)
	defer cancel()

	sa.mutex.Lock()
	defer sa.mutex.Unlock()

	if sa.buffer.Len() > 0 {
		sa.processBuffer(tempCtx)
		return
	}
}

func (sa *StringAccumulator) Shutdown() {
	sa.wg.Wait()
}
