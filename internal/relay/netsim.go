package relay

import (
	"container/heap"
	"fmt"
	"math/rand/v2"
	"os"
	"sync"
	"time"
)

const (
	maxProfileMs      = 5000
	maxProfilePercent = 100
	maxRetransmits    = 5
	maxQueuedFrames   = 10000
)

// Profile is the network condition simulated on a client's link. The same
// values apply in both directions, so the added round trip is 2 x LatencyMs.
type Profile struct {
	LatencyMs   int     `json:"latencyMs"`
	JitterMs    int     `json:"jitterMs"`
	LossPercent float64 `json:"lossPercent"`
}

func (p Profile) Validate() error {
	if p.LatencyMs < 0 || p.LatencyMs > maxProfileMs {
		return fmt.Errorf("latencyMs must be between 0 and %d", maxProfileMs)
	}
	if p.JitterMs < 0 || p.JitterMs > maxProfileMs {
		return fmt.Errorf("jitterMs must be between 0 and %d", maxProfileMs)
	}
	if p.LossPercent < 0 || p.LossPercent > maxProfilePercent {
		return fmt.Errorf("lossPercent must be between 0 and %d", maxProfilePercent)
	}
	return nil
}

func (p Profile) isOff() bool {
	return p.LatencyMs == 0 && p.JitterMs == 0 && p.LossPercent == 0
}

// channelIsReliable and channelIsOrdered follow the delivery guarantee of each
// index in channelLabels.
func channelIsReliable(index int) bool { return index < 2 }

func channelIsOrdered(index int) bool { return index == 0 || index == 2 }

type queuedFrame struct {
	release time.Time
	seq     uint64
	deliver func()
}

type frameQueue []queuedFrame

func (queue frameQueue) Len() int { return len(queue) }
func (queue frameQueue) Less(left, right int) bool {
	if queue[left].release.Equal(queue[right].release) {
		return queue[left].seq < queue[right].seq
	}
	return queue[left].release.Before(queue[right].release)
}
func (queue frameQueue) Swap(left, right int) { queue[left], queue[right] = queue[right], queue[left] }
func (queue *frameQueue) Push(item any)       { *queue = append(*queue, item.(queuedFrame)) }
func (queue *frameQueue) Pop() any {
	old := *queue
	last := old[len(old)-1]
	*queue = old[:len(old)-1]
	return last
}

// link delays and drops the frames of one client according to its profile.
// Frames are released by a single worker in release-time order, which is what
// keeps ordered channels ordered.
type link struct {
	random func() float64

	mu          sync.Mutex
	profile     Profile
	lastRelease [channelCount]time.Time
	queue       frameQueue
	nextSeq     uint64
	delivering  bool
	warnedFull  bool
	closed      bool
	started     bool
	wake        chan struct{}
	done        chan struct{}
}

func newLink(profile Profile) *link {
	return &link{
		random:  rand.Float64,
		profile: profile,
		wake:    make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
}

func (l *link) setProfile(profile Profile) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.profile = profile
}

func (l *link) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	l.closed = true
	l.queue = nil
	close(l.done)
}

// plan decides when a frame on the channel is released, or that it is dropped.
// The caller must hold l.mu.
func (l *link) plan(channel int, now time.Time) (time.Time, bool) {
	profile := l.profile
	reliable := channelIsReliable(channel)

	lost := profile.LossPercent > 0 && l.random()*100 < profile.LossPercent
	if lost && !reliable {
		return time.Time{}, true
	}

	delayMs := float64(profile.LatencyMs)
	if profile.JitterMs > 0 {
		delayMs += (l.random()*2 - 1) * float64(profile.JitterMs)
	}
	if lost {
		// A reliable channel retransmits, which costs one round trip per loss.
		for retransmits := 0; retransmits < maxRetransmits; retransmits++ {
			delayMs += 2 * float64(profile.LatencyMs)
			if l.random()*100 >= profile.LossPercent {
				break
			}
		}
	}
	if delayMs < 0 {
		delayMs = 0
	}

	release := now.Add(time.Duration(delayMs * float64(time.Millisecond)))
	if channelIsOrdered(channel) {
		if release.Before(l.lastRelease[channel]) {
			release = l.lastRelease[channel]
		}
		l.lastRelease[channel] = release
	}
	return release, false
}

// submit calls deliver for the frame once its simulated delay has passed, or
// never when the frame is lost.
func (l *link) submit(channel int, deliver func()) {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}
	if l.profile.isOff() && len(l.queue) == 0 && !l.delivering {
		l.mu.Unlock()
		deliver()
		return
	}
	if len(l.queue) >= maxQueuedFrames && !channelIsReliable(channel) {
		if !l.warnedFull {
			l.warnedFull = true
			fmt.Fprintf(os.Stderr, "network simulation queue is full, dropping unreliable frames\n")
		}
		l.mu.Unlock()
		return
	}

	release, dropped := l.plan(channel, time.Now())
	if dropped {
		l.mu.Unlock()
		return
	}
	heap.Push(&l.queue, queuedFrame{release: release, seq: l.nextSeq, deliver: deliver})
	l.nextSeq++
	if !l.started {
		l.started = true
		go l.run()
	}
	l.mu.Unlock()

	select {
	case l.wake <- struct{}{}:
	default:
	}
}

func (l *link) run() {
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		l.mu.Lock()
		if l.closed {
			l.mu.Unlock()
			return
		}
		now := time.Now()
		var due []queuedFrame
		for len(l.queue) > 0 && !l.queue[0].release.After(now) {
			due = append(due, heap.Pop(&l.queue).(queuedFrame))
		}
		wait := time.Duration(-1)
		if len(l.queue) > 0 {
			wait = l.queue[0].release.Sub(now)
		}
		l.delivering = len(due) > 0
		l.mu.Unlock()

		for _, frame := range due {
			frame.deliver()
		}
		if len(due) > 0 {
			l.mu.Lock()
			l.delivering = false
			l.mu.Unlock()
			continue
		}

		if wait < 0 {
			select {
			case <-l.wake:
			case <-l.done:
				return
			}
			continue
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(wait)
		select {
		case <-timer.C:
		case <-l.wake:
		case <-l.done:
			return
		}
	}
}
